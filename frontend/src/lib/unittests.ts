export interface UnitTestQualifiedName {
  cls: string
  method: string
}

export function parseUnitTestQualifiedName(qn: string): UnitTestQualifiedName {
  const parts = String(qn || '').split('.')
  return { cls: parts[0] || 'TestCase', method: parts[1] || 'test_case' }
}

export function leadingIndent(line: string): string {
  const match = line.match(/^[\t ]*/)
  return match ? match[0] : ''
}

export function stripUnittestMainBlock(src: string): string {
  const lines = String(src ?? '').split('\n')
  const guardIndex = lines.findIndex((line) => line.trim().match(/^if\s+__name__\s*==\s*['\"]__main__['\"]\s*:/))
  const slice = guardIndex >= 0 ? lines.slice(0, guardIndex) : lines.slice()
  while (slice.length && slice[slice.length - 1].trim() === '') {
    slice.pop()
  }
  return slice.join('\n')
}

function escapeRegExp(value: string): string {
  return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

export function extractMethodFromUnittest(src: string, qn: string): string {
  try {
    const { cls, method } = parseUnitTestQualifiedName(qn)
    const escapedClass = escapeRegExp(cls)
    const escapedMethod = escapeRegExp(method)
    const lines = String(src || '').split('\n')
    const classRE = new RegExp(`^([\\t ]*)class\\s+${escapedClass}\\s*\\(.*unittest\\.TestCase.*\\):`)
    const methodRE = new RegExp(`^([\\t ]*)def\\s+${escapedMethod}\\s*\\(`)
    let classIdx = -1
    let classIndent = ''

    for (let i = 0; i < lines.length; i++) {
      const match = lines[i].match(classRE)
      if (match) {
        classIdx = i
        classIndent = match[1] || ''
        break
      }
    }
    if (classIdx === -1) return ''

    let start = -1
    let startIndent = ''
    for (let i = classIdx + 1; i < lines.length; i++) {
      const line = lines[i]
      if (line.trim() === '') continue
      if (!line.startsWith(classIndent) && leadingIndent(line).length <= classIndent.length) break
      const match = line.match(methodRE)
      if (match) {
        start = i
        startIndent = match[1] || ''
        while (
          start - 1 > classIdx &&
          lines[start - 1].trim().startsWith('@') &&
          leadingIndent(lines[start - 1]) === startIndent
        ) {
          start--
        }
        break
      }
    }
    if (start === -1) return ''

    let end = lines.length
    for (let i = start + 1; i < lines.length; i++) {
      const line = lines[i]
      if (line.trim() === '') continue
      const indent = leadingIndent(line)
      const trimmed = line.trimStart()
      if (indent.length <= startIndent.length && (trimmed.startsWith('def ') || trimmed.startsWith('class '))) {
        end = i
        break
      }
    }

    return stripUnittestMainBlock(lines.slice(start, end).join('\n'))
  } catch (e) {
    return ''
  }
}
