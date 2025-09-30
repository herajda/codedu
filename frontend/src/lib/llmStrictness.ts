export type StrictnessDescriptor = {
  threshold: number
  message: string
}

export const STRICTNESS_DESCRIPTORS: readonly StrictnessDescriptor[] = [
  { threshold: 0, message: 'Focus only on the most basic happy-path scenario; ignore edge cases.' },
  { threshold: 5, message: 'Focus on the main happy-path scenario; minimal error handling.' },
  { threshold: 10, message: 'Test happy-path scenarios and basic error handling.' },
  { threshold: 15, message: 'Test happy-path scenarios and a few common error cases.' },
  { threshold: 20, message: 'Test happy-path scenarios and some error handling.' },
  { threshold: 25, message: 'Test happy-path scenarios and check for basic robustness.' },
  { threshold: 30, message: 'Focus on representative happy-path scenarios while checking fundamental error handling.' },
  { threshold: 35, message: 'Test typical flows and some important edge cases.' },
  { threshold: 40, message: 'Test typical flows and several edge cases.' },
  { threshold: 45, message: 'Balance typical flows with a few edge cases and robustness checks.' },
  { threshold: 50, message: 'Balance typical flows with important edge cases and robustness checks.' },
  { threshold: 55, message: 'Balance typical flows with more edge cases and robustness checks.' },
  { threshold: 60, message: 'Balance typical flows with thorough edge cases and robustness checks.' },
  { threshold: 65, message: 'Balance typical flows with comprehensive edge cases and robustness checks.' },
  { threshold: 70, message: 'Balance typical flows with important edge cases and robustness checks.' },
  { threshold: 75, message: 'Be strict and adversarial, probing tricky edge cases and robustness.' },
  { threshold: 80, message: 'Be strict and adversarial, probing more tricky edge cases and robustness.' },
  { threshold: 85, message: 'Be strict and adversarial, probing all tricky edge cases and robustness.' },
  { threshold: 90, message: 'Be strict and adversarial, probing tricky edge cases and robustness.' },
  { threshold: 95, message: 'Be maximally adversarial and exhaustive across edge cases.' },
  { threshold: 100, message: 'Be maximally adversarial and exhaustive across edge cases.' }
] as const

export function strictnessGuidance(value: number): string {
  const clamped = Math.max(0, Math.min(100, Math.round(value)))
  let message = STRICTNESS_DESCRIPTORS[0].message
  for (const descriptor of STRICTNESS_DESCRIPTORS) {
    if (clamped >= descriptor.threshold) {
      message = descriptor.message
    } else {
      break
    }
  }
  return message
}
