package main

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

type ScratchProject struct {
	Targets []ScratchTarget `json:"targets"`
}

type ScratchCostume struct {
	AssetID    string `json:"assetId"`
	Name       string `json:"name"`
	MD5Ext     string `json:"md5ext"`
	DataFormat string `json:"dataFormat"`
}

type ScratchTarget struct {
	Name           string                   `json:"name"`
	IsStage        bool                     `json:"isStage"`
	Blocks         map[string]*ScratchBlock `json:"blocks"`
	Variables      map[string][]any         `json:"variables"`
	Lists          map[string][]any         `json:"lists"`
	Costumes       []ScratchCostume         `json:"costumes"`
	CurrentCostume int                      `json:"currentCostume"`
}

type ScratchBlock struct {
	Opcode   string           `json:"opcode"`
	Next     string           `json:"next"`
	Parent   string           `json:"parent"`
	Inputs   map[string]any   `json:"inputs"`
	Fields   map[string][]any `json:"fields"`
	Shadow   bool             `json:"shadow"`
	TopLevel bool             `json:"topLevel"`
	X        *float64         `json:"x"`
	Y        *float64         `json:"y"`
	Mutation map[string]any   `json:"mutation"`
}

type ScratchSerializerOptions struct {
	MaxChars            int
	MaxScriptsPerTarget int
}

type ScratchSerializeStats struct {
	Truncated       bool
	IncludedTargets int
	OmittedTargets  int
}

const (
	defaultScratchMaxChars            = 12000
	defaultScratchMaxScriptsPerTarget = 50
)

func SerializeScratchProject(project *ScratchProject, opts ScratchSerializerOptions) (string, ScratchSerializeStats) {
	stats := ScratchSerializeStats{}
	if project == nil {
		return "", stats
	}
	maxChars := opts.MaxChars
	if maxChars <= 0 {
		maxChars = defaultScratchMaxChars
	}
	maxScripts := opts.MaxScriptsPerTarget
	if maxScripts <= 0 {
		maxScripts = defaultScratchMaxScriptsPerTarget
	}

	type targetInfo struct {
		target      *ScratchTarget
		scriptCount int
		nameKey     string
	}

	infos := make([]targetInfo, 0, len(project.Targets))
	for i := range project.Targets {
		t := &project.Targets[i]
		if t == nil {
			continue
		}
		infos = append(infos, targetInfo{
			target:      t,
			scriptCount: countTopLevelScripts(t),
			nameKey:     strings.ToLower(strings.TrimSpace(t.Name)),
		})
	}

	sort.Slice(infos, func(i, j int) bool {
		if infos[i].target.IsStage != infos[j].target.IsStage {
			return infos[i].target.IsStage
		}
		if infos[i].scriptCount != infos[j].scriptCount {
			return infos[i].scriptCount > infos[j].scriptCount
		}
		return infos[i].nameKey < infos[j].nameKey
	})

	var sb strings.Builder
	serializer := scratchSerializer{inlineDepthLimit: 6}
	for idx, info := range infos {
		serializer.blocks = info.target.Blocks
		lines := serializer.serializeTarget(info.target, maxScripts)
		if len(lines) == 0 {
			continue
		}
		block := strings.Join(lines, "\n")
		sep := ""
		if sb.Len() > 0 {
			sep = "\n\n"
		}
		if maxChars > 0 && sb.Len()+len(sep)+len(block) > maxChars {
			if sb.Len() == 0 {
				trimmed := block
				if len(trimmed) > maxChars {
					trimmed = trimmed[:maxChars]
				}
				sb.WriteString(trimmed)
				stats.IncludedTargets++
			}
			stats.Truncated = true
			stats.OmittedTargets = len(infos) - idx - 1
			break
		}
		sb.WriteString(sep)
		sb.WriteString(block)
		stats.IncludedTargets++
	}

	if stats.Truncated && stats.OmittedTargets > 0 {
		note := fmt.Sprintf("\n\nNOTE: Output truncated; omitted %d sprite(s).", stats.OmittedTargets)
		if maxChars <= 0 || sb.Len()+len(note) <= maxChars {
			sb.WriteString(note)
		}
	}

	return sb.String(), stats
}

type scratchSerializer struct {
	blocks           map[string]*ScratchBlock
	inlineDepthLimit int
}

func (s *scratchSerializer) serializeTarget(target *ScratchTarget, maxScripts int) []string {
	if target == nil {
		return nil
	}

	vars := extractNameList(target.Variables)
	lists := extractNameList(target.Lists)
	topLevel := topLevelBlocks(target)
	if len(topLevel) == 0 && len(vars) == 0 && len(lists) == 0 {
		return nil
	}

	label := "Sprite"
	if target.IsStage {
		label = "Stage"
	}
	name := strings.TrimSpace(target.Name)
	if name == "" {
		name = "(unnamed)"
	}
	lines := []string{fmt.Sprintf("%s: %s", label, name)}
	costumeNames := extractCostumeNames(target.Costumes)
	if len(costumeNames) > 0 {
		if target.IsStage {
			lines = append(lines, formatMetaLine("Backdrops", costumeNames))
		} else {
			lines = append(lines, formatMetaLine("Costumes", costumeNames))
		}
	}
	if len(vars) > 0 {
		lines = append(lines, formatMetaLine("Variables", vars))
	}
	if len(lists) > 0 {
		lines = append(lines, formatMetaLine("Lists", lists))
	}

	if maxScripts > 0 && len(topLevel) > maxScripts {
		omitted := len(topLevel) - maxScripts
		topLevel = topLevel[:maxScripts]
		for _, id := range topLevel {
			lines = append(lines, s.renderStack(id, 0, map[string]bool{})...)
		}
		lines = append(lines, formatIndentedLine(0, fmt.Sprintf("... (%d more scripts omitted)", omitted)))
		return lines
	}

	for _, id := range topLevel {
		lines = append(lines, s.renderStack(id, 0, map[string]bool{})...)
	}
	return lines
}

func extractCostumeNames(costumes []ScratchCostume) []string {
	if len(costumes) == 0 {
		return nil
	}
	names := make([]string, 0, len(costumes))
	for _, c := range costumes {
		name := strings.TrimSpace(c.Name)
		if name != "" {
			names = append(names, name)
		}
	}
	return names
}

func extractNameList(input map[string][]any) []string {
	if len(input) == 0 {
		return nil
	}
	seen := make(map[string]struct{})
	for _, raw := range input {
		if len(raw) == 0 {
			continue
		}
		name, ok := raw[0].(string)
		if !ok {
			continue
		}
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		seen[name] = struct{}{}
	}
	if len(seen) == 0 {
		return nil
	}
	list := make([]string, 0, len(seen))
	for name := range seen {
		list = append(list, name)
	}
	sort.Slice(list, func(i, j int) bool {
		return strings.ToLower(list[i]) < strings.ToLower(list[j])
	})
	return list
}

func formatMetaLine(label string, values []string) string {
	return formatIndentedLine(0, fmt.Sprintf("%s: %s", label, strings.Join(values, ", ")))
}

func topLevelBlocks(target *ScratchTarget) []string {
	if target == nil || len(target.Blocks) == 0 {
		return nil
	}

	type candidate struct {
		id     string
		x      float64
		y      float64
		opcode string
	}

	candidates := make([]candidate, 0)
	for id, block := range target.Blocks {
		if block == nil || block.Opcode == "" || block.Shadow {
			continue
		}
		if block.Parent != "" {
			continue
		}
		x := 0.0
		y := 0.0
		if block.X != nil {
			x = *block.X
		}
		if block.Y != nil {
			y = *block.Y
		}
		candidates = append(candidates, candidate{id: id, x: x, y: y, opcode: block.Opcode})
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].y != candidates[j].y {
			return candidates[i].y < candidates[j].y
		}
		if candidates[i].x != candidates[j].x {
			return candidates[i].x < candidates[j].x
		}
		if candidates[i].opcode != candidates[j].opcode {
			return candidates[i].opcode < candidates[j].opcode
		}
		return candidates[i].id < candidates[j].id
	})

	ids := make([]string, 0, len(candidates))
	for _, c := range candidates {
		ids = append(ids, c.id)
	}
	return ids
}

func countTopLevelScripts(target *ScratchTarget) int {
	if target == nil || len(target.Blocks) == 0 {
		return 0
	}
	count := 0
	for _, block := range target.Blocks {
		if block == nil || block.Opcode == "" || block.Shadow {
			continue
		}
		if block.Parent != "" {
			continue
		}
		count++
	}
	return count
}

func (s *scratchSerializer) renderStack(startID string, indent int, visited map[string]bool) []string {
	lines := []string{}
	id := startID
	for id != "" {
		if visited[id] {
			lines = append(lines, formatIndentedLine(indent, "..."))
			break
		}
		visited[id] = true
		block := s.blocks[id]
		if block == nil || block.Opcode == "" {
			break
		}
		if block.Shadow {
			id = block.Next
			continue
		}
		blockLines, consumeNext := s.renderBlockLines(block, indent)
		lines = append(lines, blockLines...)
		if consumeNext {
			break
		}
		id = block.Next
	}
	return lines
}

func (s *scratchSerializer) renderBlockLines(block *ScratchBlock, indent int) ([]string, bool) {
	if block == nil {
		return nil, false
	}

	if block.Opcode == "procedures_definition" {
		name := s.procedureDefinitionName(block)
		lines := []string{formatIndentedLine(indent, fmt.Sprintf("DEFINE %s:", name))}
		lines = append(lines, s.renderStack(block.Next, indent+1, map[string]bool{})...)
		return lines, true
	}

	if isHatBlock(block.Opcode) {
		line := s.renderStatement(block, 0)
		lines := []string{formatIndentedLine(indent, line)}
		lines = append(lines, s.renderStack(block.Next, indent+1, map[string]bool{})...)
		return lines, true
	}

	switch block.Opcode {
	case "control_repeat":
		times := fallbackValue(s.inputValue(block, "TIMES", 0))
		lines := []string{formatIndentedLine(indent, fmt.Sprintf("REPEAT %s:", times))}
		lines = append(lines, s.renderSubstack(block, "SUBSTACK", indent+1)...)
		return lines, false
	case "control_forever":
		lines := []string{formatIndentedLine(indent, "FOREVER:")}
		lines = append(lines, s.renderSubstack(block, "SUBSTACK", indent+1)...)
		return lines, false
	case "control_if":
		cond := fallbackValue(s.inputValue(block, "CONDITION", 0))
		lines := []string{formatIndentedLine(indent, fmt.Sprintf("IF %s:", cond))}
		lines = append(lines, s.renderSubstack(block, "SUBSTACK", indent+1)...)
		return lines, false
	case "control_if_else":
		cond := fallbackValue(s.inputValue(block, "CONDITION", 0))
		lines := []string{formatIndentedLine(indent, fmt.Sprintf("IF %s:", cond))}
		lines = append(lines, s.renderSubstack(block, "SUBSTACK", indent+1)...)
		lines = append(lines, formatIndentedLine(indent, "ELSE:"))
		lines = append(lines, s.renderSubstack(block, "SUBSTACK2", indent+1)...)
		return lines, false
	case "control_repeat_until":
		cond := fallbackValue(s.inputValue(block, "CONDITION", 0))
		lines := []string{formatIndentedLine(indent, fmt.Sprintf("REPEAT UNTIL %s:", cond))}
		lines = append(lines, s.renderSubstack(block, "SUBSTACK", indent+1)...)
		return lines, false
	}

	return []string{formatIndentedLine(indent, s.renderStatement(block, 0))}, false
}

func (s *scratchSerializer) renderSubstack(block *ScratchBlock, inputName string, indent int) []string {
	id := s.inputBlockID(block, inputName)
	if id == "" {
		return nil
	}
	return s.renderStack(id, indent, map[string]bool{})
}

func (s *scratchSerializer) renderStatement(block *ScratchBlock, depth int) string {
	switch block.Opcode {
	case "event_whenflagclicked":
		return "WHEN GREEN FLAG CLICKED:"
	case "event_whenkeypressed":
		key := s.fieldValue(block, "KEY_OPTION")
		if key == "" {
			key = s.inputValue(block, "KEY_OPTION", depth)
		}
		return fmt.Sprintf("WHEN [%s] KEY PRESSED:", formatKeyLabel(key))
	case "event_whenthisspriteclicked":
		return "WHEN THIS SPRITE CLICKED:"
	case "event_whenstageclicked":
		return "WHEN STAGE CLICKED:"
	case "event_whenbackdropswitchesto":
		backdrop := s.fieldValue(block, "BACKDROP")
		if backdrop == "" {
			backdrop = s.inputValue(block, "BACKDROP", depth)
		}
		return fmt.Sprintf("WHEN BACKDROP SWITCHES TO [%s]:", fallbackValue(backdrop))
	case "event_whenbroadcastreceived":
		msg := s.fieldValue(block, "BROADCAST_OPTION")
		if msg == "" {
			msg = s.inputValue(block, "BROADCAST_OPTION", depth)
		}
		return fmt.Sprintf("WHEN I RECEIVE [%s]:", fallbackValue(msg))
	case "event_whengreaterthan":
		sensor := s.fieldValue(block, "WHENGREATERTHANMENU")
		val := s.inputValue(block, "VALUE", depth)
		return fmt.Sprintf("WHEN %s > %s:", formatKeyword(sensor), fallbackValue(val))
	case "control_start_as_clone":
		return "WHEN I START AS A CLONE:"
	case "event_broadcast":
		msg := s.inputValue(block, "BROADCAST_INPUT", depth)
		if msg == "" {
			msg = s.inputValue(block, "BROADCAST_OPTION", depth)
		}
		if msg == "" {
			msg = s.fieldValue(block, "BROADCAST_OPTION")
		}
		return fmt.Sprintf("BROADCAST %s", fallbackValue(msg))
	case "event_broadcastandwait":
		msg := s.inputValue(block, "BROADCAST_INPUT", depth)
		if msg == "" {
			msg = s.inputValue(block, "BROADCAST_OPTION", depth)
		}
		if msg == "" {
			msg = s.fieldValue(block, "BROADCAST_OPTION")
		}
		return fmt.Sprintf("BROADCAST %s AND WAIT", fallbackValue(msg))
	case "control_wait":
		secs := s.inputValue(block, "DURATION", depth)
		if secs == "" {
			secs = s.inputValue(block, "SECONDS", depth)
		}
		return fmt.Sprintf("WAIT %s SECS", fallbackValue(secs))
	case "control_wait_until":
		cond := s.inputValue(block, "CONDITION", depth)
		return fmt.Sprintf("WAIT UNTIL %s", fallbackValue(cond))
	case "control_stop":
		option := s.fieldValue(block, "STOP_OPTION")
		if option == "" {
			option = s.inputValue(block, "STOP_OPTION", depth)
		}
		return fmt.Sprintf("STOP %s", formatKeyword(option))
	case "control_create_clone_of":
		target := s.inputValue(block, "CLONE_OPTION", depth)
		return fmt.Sprintf("CREATE CLONE OF %s", fallbackValue(target))
	case "control_delete_this_clone":
		return "DELETE THIS CLONE"
	case "motion_movesteps":
		steps := s.inputValue(block, "STEPS", depth)
		return fmt.Sprintf("MOVE %s STEPS", fallbackValue(steps))
	case "motion_changexby":
		val := s.inputValue(block, "DX", depth)
		return fmt.Sprintf("CHANGE X BY %s", fallbackValue(val))
	case "motion_changeyby":
		val := s.inputValue(block, "DY", depth)
		return fmt.Sprintf("CHANGE Y BY %s", fallbackValue(val))
	case "motion_setx":
		val := s.inputValue(block, "X", depth)
		return fmt.Sprintf("SET X TO %s", fallbackValue(val))
	case "motion_sety":
		val := s.inputValue(block, "Y", depth)
		return fmt.Sprintf("SET Y TO %s", fallbackValue(val))
	case "motion_gotoxy":
		x := s.inputValue(block, "X", depth)
		y := s.inputValue(block, "Y", depth)
		return fmt.Sprintf("GO TO X:%s Y:%s", fallbackValue(x), fallbackValue(y))
	case "motion_glideto":
		secs := s.inputValue(block, "SECS", depth)
		dest := s.inputValue(block, "TO", depth)
		return fmt.Sprintf("GLIDE %s SECS TO %s", fallbackValue(secs), fallbackValue(dest))
	case "motion_glidesecstoxy":
		secs := s.inputValue(block, "SECS", depth)
		x := s.inputValue(block, "X", depth)
		y := s.inputValue(block, "Y", depth)
		return fmt.Sprintf("GLIDE %s SECS TO X:%s Y:%s", fallbackValue(secs), fallbackValue(x), fallbackValue(y))
	case "motion_turnright":
		deg := s.inputValue(block, "DEGREES", depth)
		return fmt.Sprintf("TURN RIGHT %s DEGREES", fallbackValue(deg))
	case "motion_turnleft":
		deg := s.inputValue(block, "DEGREES", depth)
		return fmt.Sprintf("TURN LEFT %s DEGREES", fallbackValue(deg))
	case "motion_pointindirection":
		deg := s.inputValue(block, "DIRECTION", depth)
		return fmt.Sprintf("POINT IN DIRECTION %s", fallbackValue(deg))
	case "motion_pointtowards":
		target := s.inputValue(block, "TOWARDS", depth)
		return fmt.Sprintf("POINT TOWARDS %s", fallbackValue(target))
	case "looks_sayforsecs":
		msg := s.inputValue(block, "MESSAGE", depth)
		secs := s.inputValue(block, "SECS", depth)
		return fmt.Sprintf("SAY %s FOR %s SECS", fallbackValue(msg), fallbackValue(secs))
	case "looks_say":
		msg := s.inputValue(block, "MESSAGE", depth)
		return fmt.Sprintf("SAY %s", fallbackValue(msg))
	case "looks_thinkforsecs":
		msg := s.inputValue(block, "MESSAGE", depth)
		secs := s.inputValue(block, "SECS", depth)
		return fmt.Sprintf("THINK %s FOR %s SECS", fallbackValue(msg), fallbackValue(secs))
	case "looks_think":
		msg := s.inputValue(block, "MESSAGE", depth)
		return fmt.Sprintf("THINK %s", fallbackValue(msg))
	case "looks_switchcostumeto":
		costume := s.inputValue(block, "COSTUME", depth)
		if costume == "" {
			costume = s.fieldValue(block, "COSTUME")
		}
		return fmt.Sprintf("SWITCH COSTUME TO %s", fallbackValue(costume))
	case "looks_nextcostume":
		return "NEXT COSTUME"
	case "looks_switchbackdropto":
		backdrop := s.inputValue(block, "BACKDROP", depth)
		if backdrop == "" {
			backdrop = s.fieldValue(block, "BACKDROP")
		}
		return fmt.Sprintf("SWITCH BACKDROP TO %s", fallbackValue(backdrop))
	case "looks_nextbackdrop":
		return "NEXT BACKDROP"
	case "looks_show":
		return "SHOW"
	case "looks_hide":
		return "HIDE"
	case "looks_changesizeby":
		val := s.inputValue(block, "CHANGE", depth)
		return fmt.Sprintf("CHANGE SIZE BY %s", fallbackValue(val))
	case "looks_setsizeto":
		val := s.inputValue(block, "SIZE", depth)
		return fmt.Sprintf("SET SIZE TO %s%%", fallbackValue(val))
	case "sound_play":
		snd := s.inputValue(block, "SOUND_MENU", depth)
		if snd == "" {
			snd = s.fieldValue(block, "SOUND_MENU")
		}
		return fmt.Sprintf("PLAY SOUND %s", fallbackValue(snd))
	case "sound_playuntildone":
		snd := s.inputValue(block, "SOUND_MENU", depth)
		if snd == "" {
			snd = s.fieldValue(block, "SOUND_MENU")
		}
		return fmt.Sprintf("PLAY SOUND %s UNTIL DONE", fallbackValue(snd))
	case "sound_stopallsounds":
		return "STOP ALL SOUNDS"
	case "data_setvariableto":
		name := s.fieldValue(block, "VARIABLE")
		val := s.inputValue(block, "VALUE", depth)
		return fmt.Sprintf("SET %s TO %s", fallbackValue(name), fallbackValue(val))
	case "data_changevariableby":
		name := s.fieldValue(block, "VARIABLE")
		val := s.inputValue(block, "VALUE", depth)
		return fmt.Sprintf("CHANGE %s BY %s", fallbackValue(name), fallbackValue(val))
	case "data_showvariable":
		name := s.fieldValue(block, "VARIABLE")
		return fmt.Sprintf("SHOW VARIABLE %s", fallbackValue(name))
	case "data_hidevariable":
		name := s.fieldValue(block, "VARIABLE")
		return fmt.Sprintf("HIDE VARIABLE %s", fallbackValue(name))
	case "data_addtolist":
		item := s.inputValue(block, "ITEM", depth)
		list := s.fieldValue(block, "LIST")
		return fmt.Sprintf("ADD %s TO %s", fallbackValue(item), fallbackValue(list))
	case "data_deleteoflist":
		index := s.inputValue(block, "INDEX", depth)
		list := s.fieldValue(block, "LIST")
		return fmt.Sprintf("DELETE %s OF %s", fallbackValue(index), fallbackValue(list))
	case "data_deletealloflist":
		list := s.fieldValue(block, "LIST")
		return fmt.Sprintf("DELETE ALL OF %s", fallbackValue(list))
	case "data_insertatlist":
		item := s.inputValue(block, "ITEM", depth)
		index := s.inputValue(block, "INDEX", depth)
		list := s.fieldValue(block, "LIST")
		return fmt.Sprintf("INSERT %s AT %s OF %s", fallbackValue(item), fallbackValue(index), fallbackValue(list))
	case "data_replaceitemoflist":
		index := s.inputValue(block, "INDEX", depth)
		item := s.inputValue(block, "ITEM", depth)
		list := s.fieldValue(block, "LIST")
		return fmt.Sprintf("REPLACE ITEM %s OF %s WITH %s", fallbackValue(index), fallbackValue(list), fallbackValue(item))
	case "data_showlist":
		list := s.fieldValue(block, "LIST")
		return fmt.Sprintf("SHOW LIST %s", fallbackValue(list))
	case "data_hidelist":
		list := s.fieldValue(block, "LIST")
		return fmt.Sprintf("HIDE LIST %s", fallbackValue(list))
	case "procedures_call":
		return s.procedureCallName(block, depth)
	}

	return s.genericStatement(block, depth)
}

func (s *scratchSerializer) renderInline(blockID string, depth int) string {
	if depth > s.inlineDepthLimit {
		return "..."
	}
	block := s.blocks[blockID]
	if block == nil {
		return ""
	}

	switch block.Opcode {
	case "operator_add":
		return fmt.Sprintf("(%s + %s)",
			fallbackValue(s.inputValue(block, "NUM1", depth+1)),
			fallbackValue(s.inputValue(block, "NUM2", depth+1)),
		)
	case "operator_subtract":
		return fmt.Sprintf("(%s - %s)",
			fallbackValue(s.inputValue(block, "NUM1", depth+1)),
			fallbackValue(s.inputValue(block, "NUM2", depth+1)),
		)
	case "operator_multiply":
		return fmt.Sprintf("(%s * %s)",
			fallbackValue(s.inputValue(block, "NUM1", depth+1)),
			fallbackValue(s.inputValue(block, "NUM2", depth+1)),
		)
	case "operator_divide":
		return fmt.Sprintf("(%s / %s)",
			fallbackValue(s.inputValue(block, "NUM1", depth+1)),
			fallbackValue(s.inputValue(block, "NUM2", depth+1)),
		)
	case "operator_mod":
		return fmt.Sprintf("(%s mod %s)",
			fallbackValue(s.inputValue(block, "NUM1", depth+1)),
			fallbackValue(s.inputValue(block, "NUM2", depth+1)),
		)
	case "operator_equals":
		return fmt.Sprintf("(%s = %s)",
			fallbackValue(s.inputValue(block, "OPERAND1", depth+1)),
			fallbackValue(s.inputValue(block, "OPERAND2", depth+1)),
		)
	case "operator_gt":
		return fmt.Sprintf("(%s > %s)",
			fallbackValue(s.inputValue(block, "OPERAND1", depth+1)),
			fallbackValue(s.inputValue(block, "OPERAND2", depth+1)),
		)
	case "operator_lt":
		return fmt.Sprintf("(%s < %s)",
			fallbackValue(s.inputValue(block, "OPERAND1", depth+1)),
			fallbackValue(s.inputValue(block, "OPERAND2", depth+1)),
		)
	case "operator_and":
		return fmt.Sprintf("(%s AND %s)",
			fallbackValue(s.inputValue(block, "OPERAND1", depth+1)),
			fallbackValue(s.inputValue(block, "OPERAND2", depth+1)),
		)
	case "operator_or":
		return fmt.Sprintf("(%s OR %s)",
			fallbackValue(s.inputValue(block, "OPERAND1", depth+1)),
			fallbackValue(s.inputValue(block, "OPERAND2", depth+1)),
		)
	case "operator_not":
		return fmt.Sprintf("NOT %s", fallbackValue(s.inputValue(block, "OPERAND", depth+1)))
	case "operator_join":
		return fmt.Sprintf("JOIN(%s, %s)",
			fallbackValue(s.inputValue(block, "STRING1", depth+1)),
			fallbackValue(s.inputValue(block, "STRING2", depth+1)),
		)
	case "operator_contains":
		return fmt.Sprintf("(%s CONTAINS %s)",
			fallbackValue(s.inputValue(block, "STRING1", depth+1)),
			fallbackValue(s.inputValue(block, "STRING2", depth+1)),
		)
	case "operator_random":
		return fmt.Sprintf("RANDOM(%s, %s)",
			fallbackValue(s.inputValue(block, "FROM", depth+1)),
			fallbackValue(s.inputValue(block, "TO", depth+1)),
		)
	case "sensing_keypressed":
		key := s.inputValue(block, "KEY_OPTION", depth+1)
		return fmt.Sprintf("KEY %s PRESSED?", formatKeyLabel(key))
	case "sensing_keyoptions":
		key := s.fieldValue(block, "KEY_OPTION")
		return formatKeyLabel(key)
	case "sensing_mousedown":
		return "MOUSE DOWN?"
	case "sensing_touchingobject":
		obj := s.inputValue(block, "TOUCHINGOBJECTMENU", depth+1)
		return fmt.Sprintf("TOUCHING %s?", fallbackValue(obj))
	case "sensing_touchingcolor":
		col := s.inputValue(block, "COLOR", depth+1)
		return fmt.Sprintf("TOUCHING COLOR %s?", fallbackValue(col))
	case "sensing_coloristouchingcolor":
		col1 := s.inputValue(block, "COLOR", depth+1)
		col2 := s.inputValue(block, "COLOR2", depth+1)
		return fmt.Sprintf("COLOR %s TOUCHING %s?", fallbackValue(col1), fallbackValue(col2))
	case "sensing_distanceto":
		obj := s.inputValue(block, "DISTANCETOMENU", depth+1)
		return fmt.Sprintf("DISTANCE TO %s", fallbackValue(obj))
	case "sensing_mousex":
		return "MOUSE X"
	case "sensing_mousey":
		return "MOUSE Y"
	case "sensing_answer":
		return "ANSWER"
	case "sensing_timer":
		return "TIMER"
	case "data_variable":
		return fallbackValue(s.fieldValue(block, "VARIABLE"))
	case "data_listcontents":
		return fallbackValue(s.fieldValue(block, "LIST"))
	case "argument_reporter_string_number":
		name := s.fieldValue(block, "VALUE")
		if name == "" {
			name = s.fieldValue(block, "NAME")
		}
		return fallbackValue(name)
	case "argument_reporter_boolean":
		name := s.fieldValue(block, "VALUE")
		if name == "" {
			name = s.fieldValue(block, "NAME")
		}
		return fallbackValue(name)
	case "looks_costumenumbername":
		val := s.fieldValue(block, "NUMBER_NAME")
		return fallbackValue(val)
	case "looks_backdropnumbername":
		val := s.fieldValue(block, "NUMBER_NAME")
		return fallbackValue(val)
	case "motion_xposition":
		return "X POSITION"
	case "motion_yposition":
		return "Y POSITION"
	}

	if value := s.inlineFallback(block, depth); value != "" {
		return value
	}

	return formatOpcode(block.Opcode)
}

func (s *scratchSerializer) inputValue(block *ScratchBlock, name string, depth int) string {
	if block == nil {
		return ""
	}
	raw, ok := block.Inputs[name]
	if !ok {
		return ""
	}
	return s.formatInput(raw, depth)
}

func (s *scratchSerializer) inputBlockID(block *ScratchBlock, name string) string {
	if block == nil {
		return ""
	}
	raw, ok := block.Inputs[name]
	if !ok {
		return ""
	}
	arr, ok := raw.([]any)
	if !ok || len(arr) < 2 {
		return ""
	}
	id, _ := arr[1].(string)
	return id
}

func (s *scratchSerializer) formatInput(raw any, depth int) string {
	switch v := raw.(type) {
	case nil:
		return ""
	case []any:
		if len(v) >= 2 {
			return s.formatInput(v[1], depth)
		}
		if len(v) == 1 {
			return s.formatInput(v[0], depth)
		}
		return ""
	case string:
		if block, ok := s.blocks[v]; ok && block != nil {
			return s.renderInline(v, depth+1)
		}
		return v
	case float64:
		return formatNumber(v)
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}

func (s *scratchSerializer) fieldValue(block *ScratchBlock, name string) string {
	if block == nil {
		return ""
	}
	vals, ok := block.Fields[name]
	if !ok || len(vals) == 0 {
		return ""
	}
	switch v := vals[0].(type) {
	case string:
		return v
	case float64:
		return formatNumber(v)
	default:
		return ""
	}
}

func (s *scratchSerializer) procedureDefinitionName(block *ScratchBlock) string {
	protoID := s.inputBlockID(block, "custom_block")
	proto := s.blocks[protoID]
	if proto == nil {
		return "custom block"
	}
	proccode := mutationString(proto.Mutation, "proccode")
	if proccode == "" {
		return "custom block"
	}
	argNames := parseJSONList(mutationString(proto.Mutation, "argumentnames"))
	for i := range argNames {
		argNames[i] = fmt.Sprintf("(%s)", argNames[i])
	}
	return formatProcCode(proccode, argNames)
}

func (s *scratchSerializer) procedureCallName(block *ScratchBlock, depth int) string {
	proccode := mutationString(block.Mutation, "proccode")
	if proccode == "" {
		return "CALL custom block"
	}
	argIDs := parseJSONList(mutationString(block.Mutation, "argumentids"))
	args := make([]string, len(argIDs))
	for i, id := range argIDs {
		args[i] = fallbackValue(s.inputValue(block, id, depth+1))
	}
	return fmt.Sprintf("CALL %s", formatProcCode(proccode, args))
}

func (s *scratchSerializer) inlineFallback(block *ScratchBlock, depth int) string {
	if block == nil {
		return ""
	}
	if len(block.Fields) == 1 {
		for _, vals := range block.Fields {
			if len(vals) > 0 {
				switch v := vals[0].(type) {
				case string:
					return v
				case float64:
					return formatNumber(v)
				}
			}
		}
	}
	if len(block.Inputs) == 1 {
		for _, raw := range block.Inputs {
			val := s.formatInput(raw, depth+1)
			if val != "" {
				return val
			}
		}
	}
	return ""
}

func (s *scratchSerializer) genericStatement(block *ScratchBlock, depth int) string {
	parts := []string{}
	for key, vals := range block.Fields {
		if len(vals) == 0 {
			continue
		}
		val := ""
		switch v := vals[0].(type) {
		case string:
			val = v
		case float64:
			val = formatNumber(v)
		}
		if val == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s=%s", key, val))
	}
	for key, raw := range block.Inputs {
		val := s.formatInput(raw, depth+1)
		if val == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s=%s", key, val))
	}
	sort.Strings(parts)
	if len(parts) > 0 {
		return fmt.Sprintf("%s (%s)", formatOpcode(block.Opcode), strings.Join(parts, ", "))
	}
	return formatOpcode(block.Opcode)
}

func formatIndentedLine(indent int, text string) string {
	if text == "" {
		text = "..."
	}
	return strings.Repeat("  ", indent) + "- " + text
}

func fallbackValue(val string) string {
	if strings.TrimSpace(val) == "" {
		return "?"
	}
	return val
}

func formatNumber(value float64) string {
	if math.Mod(value, 1) == 0 {
		return strconv.FormatInt(int64(value), 10)
	}
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func formatKeyLabel(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return "?"
	}
	parts := strings.Fields(key)
	for i := range parts {
		parts[i] = titleCase(parts[i])
	}
	return strings.Join(parts, " ")
}

func titleCase(val string) string {
	val = strings.TrimSpace(val)
	if val == "" {
		return val
	}
	lower := strings.ToLower(val)
	return strings.ToUpper(lower[:1]) + lower[1:]
}

func formatOpcode(opcode string) string {
	opcode = strings.TrimSpace(opcode)
	if opcode == "" {
		return "BLOCK"
	}
	parts := strings.Split(opcode, "_")
	for i := range parts {
		parts[i] = strings.ToUpper(parts[i])
	}
	return strings.Join(parts, " ")
}

func formatKeyword(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "?"
	}
	return strings.ToUpper(value)
}

func isHatBlock(opcode string) bool {
	switch opcode {
	case "event_whenflagclicked",
		"event_whenkeypressed",
		"event_whenthisspriteclicked",
		"event_whenstageclicked",
		"event_whenbackdropswitchesto",
		"event_whenbroadcastreceived",
		"event_whengreaterthan",
		"control_start_as_clone":
		return true
	default:
		return false
	}
}

func mutationString(mutation map[string]any, key string) string {
	if mutation == nil {
		return ""
	}
	val, ok := mutation[key]
	if !ok {
		return ""
	}
	out, _ := val.(string)
	return out
}

func parseJSONList(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var list []string
	if err := json.Unmarshal([]byte(raw), &list); err != nil {
		return nil
	}
	return list
}

func formatProcCode(proccode string, args []string) string {
	if proccode == "" {
		return "custom block"
	}
	var b strings.Builder
	argIndex := 0
	for i := 0; i < len(proccode); i++ {
		if proccode[i] == '%' && i+1 < len(proccode) {
			next := proccode[i+1]
			if next == 's' || next == 'b' {
				arg := "?"
				if argIndex < len(args) && strings.TrimSpace(args[argIndex]) != "" {
					arg = args[argIndex]
				}
				b.WriteString(arg)
				argIndex++
				i++
				continue
			}
		}
		b.WriteByte(proccode[i])
	}
	out := strings.TrimSpace(b.String())
	if out == "" {
		return proccode
	}
	return out
}
