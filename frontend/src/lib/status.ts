import { t } from "$lib/i18n";

export function submissionStatusLabel(status: string): string {
  if (status === "partially_completed") {
    return t("frontend/src/lib/status.ts::partially_completed_label");
  }
  return status;
}
