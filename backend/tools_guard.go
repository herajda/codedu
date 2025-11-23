package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type illegalToolFinding struct {
	Kind   string `json:"kind"`
	Name   string `json:"name"`
	Symbol string `json:"symbol"`
	File   string `json:"file"`
	Line   int    `json:"line"`
	Column int    `json:"column"`
}

func detectIllegalToolUse(root string, bannedFunctions, bannedModules []string) ([]illegalToolFinding, error) {
	funcs := sanitizeList(bannedFunctions)
	mods := sanitizeList(bannedModules)
	if len(funcs) == 0 && len(mods) == 0 {
		return nil, nil
	}

	funcJSON, err := json.Marshal(funcs)
	if err != nil {
		return nil, fmt.Errorf("marshal banned functions: %w", err)
	}
	modJSON, err := json.Marshal(mods)
	if err != nil {
		return nil, fmt.Errorf("marshal banned modules: %w", err)
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		absRoot = root
	}

	pythonExec := "python3"
	if _, err := exec.LookPath(pythonExec); err != nil {
		pythonExec = "python"
	}

	cmd := exec.Command(pythonExec, "-c", illegalToolAnalyzerScript, string(funcJSON), string(modJSON), absRoot)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("illegal tool analysis failed: %w (stderr: %s)", err, strings.TrimSpace(stderr.String()))
	}

	var raw []illegalToolFinding
	if err := json.Unmarshal(stdout.Bytes(), &raw); err != nil {
		return nil, fmt.Errorf("illegal tool parse: %w", err)
	}

	unique := make(map[string]illegalToolFinding, len(raw))
	list := make([]illegalToolFinding, 0, len(raw))
	for _, f := range raw {
		f.File = filepath.ToSlash(strings.TrimSpace(f.File))
		key := fmt.Sprintf("%s|%s|%s|%s|%d|%d", strings.ToLower(f.Kind), strings.ToLower(f.Name), strings.ToLower(f.Symbol), f.File, f.Line, f.Column)
		if _, exists := unique[key]; exists {
			continue
		}
		unique[key] = f
		list = append(list, f)
	}

	sort.Slice(list, func(i, j int) bool {
		if list[i].File != list[j].File {
			return list[i].File < list[j].File
		}
		if list[i].Line != list[j].Line {
			return list[i].Line < list[j].Line
		}
		if list[i].Column != list[j].Column {
			return list[i].Column < list[j].Column
		}
		if list[i].Kind != list[j].Kind {
			return list[i].Kind < list[j].Kind
		}
		return list[i].Name < list[j].Name
	})

	return list, nil
}

func formatIllegalToolMessage(findings []illegalToolFinding, notes map[string]string) string {
	if len(findings) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("Illegal tool use detected:\n")
	for _, f := range findings {
		symbol := strings.TrimSpace(f.Symbol)
		if symbol == "" {
			symbol = f.Name
		}
		action := "used banned item"
		switch f.Kind {
		case "module_import":
			action = "imported banned module"
		case "function_import":
			action = "imported banned function"
		case "function_call":
			action = "called banned function"
		}
		line := f.Line
		if line < 0 {
			line = 0
		}
		fmt.Fprintf(&b, " - %s %q (rule %q) at %s:%d", action, symbol, f.Name, f.File, line)
		if notes != nil {
			if note := strings.TrimSpace(notes[strings.ToLower(f.Name)]); note != "" {
				fmt.Fprintf(&b, " — Reason: %s", note)
			}
		}
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func sanitizeList(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))
	for _, raw := range items {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

const illegalToolAnalyzerScript = `import ast
import json
import pathlib
import sys

funcs = json.loads(sys.argv[1])
mods = json.loads(sys.argv[2])
root = pathlib.Path(sys.argv[3])

func_exact = {}
func_simple = {}
func_prefixes = []
func_suffixes = []
func_suffix_seen = set()
for item in funcs:
    if not isinstance(item, str):
        continue
    name = item.strip()
    if not name:
        continue
    lower = name.lower()
    if lower.startswith('*.'):
        suffix = lower[1:]
        if suffix and suffix not in func_suffix_seen:
            func_suffixes.append((suffix, name))
            func_suffix_seen.add(suffix)
        continue
    if lower.startswith('.'):
        if lower not in func_suffix_seen:
            func_suffixes.append((lower, name))
            func_suffix_seen.add(lower)
        continue
    if lower.endswith('.*'):
        base = lower[:-2].strip()
        if base:
            func_prefixes.append((base + '.', name))
        continue
    func_exact[lower] = name
    if '.' not in lower:
        if lower not in func_simple:
            func_simple[lower] = name
    elif lower.startswith('builtins.'):
        simple = lower.rsplit('.', 1)[-1]
        if simple and simple not in func_simple:
            func_simple[simple] = name

module_map = {}
for item in mods:
    if not isinstance(item, str):
        continue
    name = item.strip()
    if not name:
        continue
    lower = name.lower()
    if lower.endswith('.*'):
        base = lower[:-2].strip()
        if base:
            module_map.setdefault(base, name)
        continue
    module_map[lower] = name


def resolve_name(node):
    if isinstance(node, ast.Name):
        return node.id
    if isinstance(node, ast.Attribute):
        parts = []
        current = node
        while isinstance(current, ast.Attribute):
            parts.append(current.attr)
            current = current.value
        if isinstance(current, ast.Name):
            parts.append(current.id)
            return '.'.join(reversed(parts))
    return None


results = []


def add_result(kind, rule_name, symbol, rel_path, node):
    line = getattr(node, 'lineno', 0)
    col = getattr(node, 'col_offset', -1)
    results.append({
        'kind': kind,
        'name': rule_name,
        'symbol': symbol,
        'file': rel_path,
        'line': int(line),
        'column': int(col) + 1 if col >= 0 else 0,
    })


if not root.exists():
    print('[]')
    sys.exit(0)


py_files = sorted(p for p in root.rglob('*.py') if p.is_file())


for path in py_files:
    try:
        rel = path.relative_to(root).as_posix()
    except Exception:
        rel = path.name
    try:
        source = path.read_text(encoding='utf-8')
    except Exception:
        continue
    try:
        tree = ast.parse(source, filename=str(path))
    except SyntaxError:
        continue

    local_defs = set()
    assigned_names = set()

    def record_target(target):
        if isinstance(target, ast.Name):
            assigned_names.add(target.id.lower())
        elif isinstance(target, (ast.Tuple, ast.List)):
            for elt in target.elts:
                record_target(elt)

    class DefinitionTracker(ast.NodeVisitor):
        def visit_FunctionDef(self, node):
            local_defs.add(node.name.lower())
            self.generic_visit(node)

        def visit_AsyncFunctionDef(self, node):
            local_defs.add(node.name.lower())
            self.generic_visit(node)

        def visit_Assign(self, node):
            for target in node.targets:
                record_target(target)
            self.generic_visit(node)

        def visit_AnnAssign(self, node):
            record_target(node.target)
            self.generic_visit(node)

        def visit_AugAssign(self, node):
            record_target(node.target)
            self.generic_visit(node)

    DefinitionTracker().visit(tree)

    class Visitor(ast.NodeVisitor):
        def visit_Import(self, node):
            for alias in node.names:
                target = alias.name
                lower = target.lower()
                for mod_key, mod_name in module_map.items():
                    if lower == mod_key or lower.startswith(mod_key + '.'):
                        display = target
                        if alias.asname:
                            display = f"{target} as {alias.asname}"
                        add_result('module_import', mod_name, display, rel, node)
                        break
            self.generic_visit(node)

        def visit_ImportFrom(self, node):
            module = (node.module or '').lower()
            module_display = node.module or ''
            for alias in node.names:
                name = alias.name
                lower = name.lower()
                display = name
                if alias.asname:
                    display = f"{name} as {alias.asname}"
                full = f"{module}.{lower}" if module else lower
                if full in func_exact:
                    add_result('function_import', func_exact[full], display, rel, node)
                if lower in func_simple and '.' not in func_simple[lower].lower():
                    add_result('function_import', func_simple[lower], display, rel, node)
            if module:
                current = module
                while current:
                    if current in module_map:
                        add_result('module_import', module_map[current], module_display or module, rel, node)
                        break
                    if '.' in current:
                        current = current.rsplit('.', 1)[0]
                    else:
                        current = ''
            self.generic_visit(node)

        def visit_Call(self, node):
            name = resolve_name(node.func)
            if name:
                lower = name.lower()
                simple = lower.rsplit('.', 1)[-1]
                if simple in local_defs or simple in assigned_names:
                    self.generic_visit(node)
                    return
                matched = False
                if lower in func_exact:
                    add_result('function_call', func_exact[lower], name, rel, node)
                    matched = True
                elif simple in func_simple and '.' not in lower:
                    add_result('function_call', func_simple[simple], name, rel, node)
                    matched = True
                else:
                    for prefix, rule_name in func_prefixes:
                        if lower.startswith(prefix):
                            add_result('function_call', rule_name, name, rel, node)
                            matched = True
                            break
                if not matched:
                    for suffix, rule_name in func_suffixes:
                        if lower.endswith(suffix):
                            add_result('function_call', rule_name, name, rel, node)
                            break
            self.generic_visit(node)

    Visitor().visit(tree)


print(json.dumps(results))
`
