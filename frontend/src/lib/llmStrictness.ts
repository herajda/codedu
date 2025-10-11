import { t } from '$lib/i18n';

export type StrictnessDescriptor = {
  threshold: number
  message: string
}

export const STRICTNESS_DESCRIPTORS: readonly StrictnessDescriptor[] = [
  { threshold: 0, message: t('frontend/src/lib/llmStrictness.ts::focus_only_on_the_most_basic_happy_path_scenario_ignore_edge_cases') },
  { threshold: 5, message: t('frontend/src/lib/llmStrictness.ts::focus_on_the_main_happy_path_scenario_minimal_error_handling') },
  { threshold: 10, message: t('frontend/src/lib/llmStrictness.ts::test_happy_path_scenarios_and_basic_error_handling') },
  { threshold: 15, message: t('frontend/src/lib/llmStrictness.ts::test_happy_path_scenarios_and_a_few_common_error_cases') },
  { threshold: 20, message: t('frontend/src/lib/llmStrictness.ts::test_happy_path_scenarios_and_some_error_handling') },
  { threshold: 25, message: t('frontend/src/lib/llmStrictness.ts::test_happy_path_scenarios_and_check_for_basic_robustness') },
  { threshold: 30, message: t('frontend/src/lib/llmStrictness.ts::focus_on_representative_happy_path_scenarios_while_checking_fundamental_error_handling') },
  { threshold: 35, message: t('frontend/src/lib/llmStrictness.ts::test_typical_flows_and_some_important_edge_cases') },
  { threshold: 40, message: t('frontend/src/lib/llmStrictness.ts::test_typical_flows_and_several_edge_cases') },
  { threshold: 45, message: t('frontend/src/lib/llmStrictness.ts::balance_typical_flows_with_a_few_edge_cases_and_robustness_checks') },
  { threshold: 50, message: t('frontend/src/lib/llmStrictness.ts::balance_typical_flows_with_important_edge_cases_and_robustness_checks') },
  { threshold: 55, message: t('frontend/src/lib/llmStrictness.ts::balance_typical_flows_with_more_edge_cases_and_robustness_checks') },
  { threshold: 60, message: t('frontend/src/lib/llmStrictness.ts::balance_typical_flows_with_thorough_edge_cases_and_robustness_checks') },
  { threshold: 65, message: t('frontend/src/lib/llmStrictness.ts::balance_typical_flows_with_comprehensive_edge_cases_and_robustness_checks') },
  { threshold: 70, message: t('frontend/src/lib/llmStrictness.ts::balance_typical_flows_with_important_edge_cases_and_robustness_checks') },
  { threshold: 75, message: t('frontend/src/lib/llmStrictness.ts::be_strict_and_adversarial_probing_tricky_edge_cases_and_robustness') },
  { threshold: 80, message: t('frontend/src/lib/llmStrictness.ts::be_strict_and_adversarial_probing_more_tricky_edge_cases_and_robustness') },
  { threshold: 85, message: t('frontend/src/lib/llmStrictness.ts::be_strict_and_adversarial_probing_all_tricky_edge_cases_and_robustness') },
  { threshold: 90, message: t('frontend/src/lib/llmStrictness.ts::be_strict_and_adversarial_probing_tricky_edge_cases_and_robustness') },
  { threshold: 95, message: t('frontend/src/lib/llmStrictness.ts::be_maximally_adversarial_and_exhaustive_across_edge_cases') },
  { threshold: 100, message: t('frontend/src/lib/llmStrictness.ts::be_maximally_adversarial_and_exhaustive_across_edge_cases') }
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
