<script lang="ts">
  import {
    AlertCircle,
    AlertTriangle,
    CheckCircle2,
    FileCode,
    Sparkles,
    Trophy,
    X,
  } from "lucide-svelte";
  import { t, translator } from "$lib/i18n";
  import ScratchBlocksSnippet from "$lib/components/ScratchBlocksSnippet.svelte";

  export let analysis: any = null;
  export let error = "";

  let translate;
  $: translate = $translator;

  let helpDialog: HTMLDialogElement | undefined;
  let activeSkillKey: string | null = null;
  let activeSkillLabel = "";
  let activeHelp: SkillHelpEntry | null = null;

  const SUMMARY_KEYS = new Set([
    "average_points",
    "competence",
    "max_points",
    "total_blocks",
    "total_points",
  ]);

  type SkillScore = {
    name: string;
    label: string;
    score: number;
    max: number;
  };

  type SkillHelpLevel = {
    titleKey: string;
    bodyKey: string;
    exampleKey: string;
  };

  type SkillHelpEntry = {
    summaryKey: string;
    bulletsKeys: string[];
    levels: SkillHelpLevel[];
  };

  type ModeSummary = {
    key: string;
    label: string;
    score: number;
    max: number;
    safeMax: number;
    average: number | null;
    competence: string;
    totalBlocks: number | null;
    skills: SkillScore[];
  };

  type DeadCodeSprite = {
    sprite: string;
    scripts: { id: string; blocks: string[] }[];
  };

  type DuplicateGroup = {
    scripts: string[][];
  };

  const SKILL_HELP: Record<string, SkillHelpEntry> = {
    Abstraction: {
      summaryKey:
        "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_abstraction_summary",
      bulletsKeys: [
        "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_abstraction_bullet_1",
        "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_abstraction_bullet_2",
        "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_abstraction_bullet_3",
      ],
      levels: [
        {
          titleKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_level_1_title",
          bodyKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_abstraction_points_1",
          exampleKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_abstraction_example_1",
        },
        {
          titleKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_level_2_title",
          bodyKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_abstraction_points_2",
          exampleKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_abstraction_example_2",
        },
        {
          titleKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_level_3_title",
          bodyKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_abstraction_points_3",
          exampleKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_abstraction_example_3",
        },
        {
          titleKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_level_4_title",
          bodyKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_abstraction_points_4",
          exampleKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_abstraction_example_4",
        },
      ],
    },
    Parallelization: {
      summaryKey:
        "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_parallelization_summary",
      bulletsKeys: [
        "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_parallelization_bullet_1",
        "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_parallelization_bullet_2",
        "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_parallelization_bullet_3",
      ],
      levels: [
        {
          titleKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_level_1_title",
          bodyKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_parallelization_points_1",
          exampleKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_parallelization_example_1",
        },
        {
          titleKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_level_2_title",
          bodyKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_parallelization_points_2",
          exampleKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_parallelization_example_2",
        },
        {
          titleKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_level_3_title",
          bodyKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_parallelization_points_3",
          exampleKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_parallelization_example_3",
        },
        {
          titleKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_level_4_title",
          bodyKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_parallelization_points_4",
          exampleKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_parallelization_example_4",
        },
      ],
    },
    Logic: {
      summaryKey:
        "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_logic_summary",
      bulletsKeys: [
        "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_logic_bullet_1",
        "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_logic_bullet_2",
        "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_logic_bullet_3",
      ],
      levels: [
        {
          titleKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_level_1_title",
          bodyKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_logic_points_1",
          exampleKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_logic_example_1",
        },
        {
          titleKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_level_2_title",
          bodyKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_logic_points_2",
          exampleKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_logic_example_2",
        },
        {
          titleKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_level_3_title",
          bodyKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_logic_points_3",
          exampleKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_logic_example_3",
        },
        {
          titleKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_level_4_title",
          bodyKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_logic_points_4",
          exampleKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_logic_example_4",
        },
      ],
    },
    Synchronization: {
      summaryKey:
        "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_synchronization_summary",
      bulletsKeys: [
        "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_synchronization_bullet_1",
        "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_synchronization_bullet_2",
        "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_synchronization_bullet_3",
      ],
      levels: [
        {
          titleKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_level_1_title",
          bodyKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_synchronization_points_1",
          exampleKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_synchronization_example_1",
        },
        {
          titleKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_level_2_title",
          bodyKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_synchronization_points_2",
          exampleKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_synchronization_example_2",
        },
        {
          titleKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_level_3_title",
          bodyKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_synchronization_points_3",
          exampleKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_synchronization_example_3",
        },
        {
          titleKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_level_4_title",
          bodyKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_synchronization_points_4",
          exampleKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_synchronization_example_4",
        },
      ],
    },
    FlowControl: {
      summaryKey:
        "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_flow_control_summary",
      bulletsKeys: [
        "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_flow_control_bullet_1",
        "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_flow_control_bullet_2",
        "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_flow_control_bullet_3",
      ],
      levels: [
        {
          titleKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_level_1_title",
          bodyKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_flow_control_points_1",
          exampleKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_flow_control_example_1",
        },
        {
          titleKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_level_2_title",
          bodyKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_flow_control_points_2",
          exampleKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_flow_control_example_2",
        },
        {
          titleKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_level_3_title",
          bodyKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_flow_control_points_3",
          exampleKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_flow_control_example_3",
        },
        {
          titleKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_level_4_title",
          bodyKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_flow_control_points_4",
          exampleKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_flow_control_example_4",
        },
      ],
    },
    UserInteractivity: {
      summaryKey:
        "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_user_interactivity_summary",
      bulletsKeys: [
        "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_user_interactivity_bullet_1",
        "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_user_interactivity_bullet_2",
        "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_user_interactivity_bullet_3",
      ],
      levels: [
        {
          titleKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_level_1_title",
          bodyKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_user_interactivity_points_1",
          exampleKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_user_interactivity_example_1",
        },
        {
          titleKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_level_2_title",
          bodyKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_user_interactivity_points_2",
          exampleKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_user_interactivity_example_2",
        },
        {
          titleKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_level_3_title",
          bodyKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_user_interactivity_points_3",
          exampleKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_user_interactivity_example_3",
        },
        {
          titleKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_level_4_title",
          bodyKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_user_interactivity_points_4",
          exampleKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_user_interactivity_example_4",
        },
      ],
    },
    DataRepresentation: {
      summaryKey:
        "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_data_representation_summary",
      bulletsKeys: [
        "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_data_representation_bullet_1",
        "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_data_representation_bullet_2",
        "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_data_representation_bullet_3",
      ],
      levels: [
        {
          titleKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_level_1_title",
          bodyKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_data_representation_points_1",
          exampleKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_data_representation_example_1",
        },
        {
          titleKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_level_2_title",
          bodyKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_data_representation_points_2",
          exampleKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_data_representation_example_2",
        },
        {
          titleKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_level_3_title",
          bodyKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_data_representation_points_3",
          exampleKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_data_representation_example_3",
        },
        {
          titleKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_level_4_title",
          bodyKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_data_representation_points_4",
          exampleKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_data_representation_example_4",
        },
      ],
    },
    MathOperators: {
      summaryKey:
        "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_math_operators_summary",
      bulletsKeys: [
        "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_math_operators_bullet_1",
        "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_math_operators_bullet_2",
        "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_math_operators_bullet_3",
      ],
      levels: [
        {
          titleKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_level_1_title",
          bodyKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_math_operators_points_1",
          exampleKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_math_operators_example_1",
        },
        {
          titleKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_level_2_title",
          bodyKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_math_operators_points_2",
          exampleKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_math_operators_example_2",
        },
        {
          titleKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_level_3_title",
          bodyKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_math_operators_points_3",
          exampleKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_math_operators_example_3",
        },
        {
          titleKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_level_4_title",
          bodyKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_math_operators_points_4",
          exampleKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_math_operators_example_4",
        },
      ],
    },
    MotionOperators: {
      summaryKey:
        "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_motion_operators_summary",
      bulletsKeys: [
        "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_motion_operators_bullet_1",
        "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_motion_operators_bullet_2",
        "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_motion_operators_bullet_3",
      ],
      levels: [
        {
          titleKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_level_1_title",
          bodyKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_motion_operators_points_1",
          exampleKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_motion_operators_example_1",
        },
        {
          titleKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_level_2_title",
          bodyKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_motion_operators_points_2",
          exampleKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_motion_operators_example_2",
        },
        {
          titleKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_level_3_title",
          bodyKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_motion_operators_points_3",
          exampleKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_motion_operators_example_3",
        },
        {
          titleKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_level_4_title",
          bodyKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_motion_operators_points_4",
          exampleKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_motion_operators_example_4",
        },
      ],
    },
  };

  function asNumber(value: any): number {
    if (typeof value === "number" && Number.isFinite(value)) return value;
    if (typeof value === "string") {
      const parsed = Number(value);
      return Number.isFinite(parsed) ? parsed : 0;
    }
    return 0;
  }

  function toSnakeCase(value: string): string {
    return value
      .replace(/([a-z])([A-Z])/g, "$1_$2")
      .replace(/[\s-]+/g, "_")
      .toLowerCase();
  }

  function humanizeSkill(value: string): string {
    return value
      .replace(/([a-z])([A-Z])/g, "$1 $2")
      .replace(/_/g, " ")
      .trim();
  }

  function skillLabel(skill: string): string {
    const key = `frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_${toSnakeCase(skill)}`;
    const label = translate ? translate(key) : t(key);
    return label === key ? humanizeSkill(skill) : label;
  }

  $: activeHelp = activeSkillKey ? SKILL_HELP[activeSkillKey] ?? null : null;
  $: activeSkillLabel = (() => {
    translate;
    return activeSkillKey ? skillLabel(activeSkillKey) : "";
  })();

  function extractScore(section: any): {
    score: number;
    max: number;
    safeMax: number;
  } {
    if (!section || typeof section !== "object") {
      return { score: 0, max: 0, safeMax: 1 };
    }
    const total = section.total_points;
    let score = 0;
    let max = 0;
    if (Array.isArray(total)) {
      score = asNumber(total[0]);
      max = asNumber(total[1]);
    } else {
      score = asNumber(total);
      max = asNumber(section.max_points);
    }
    const safeMax = max > 0 ? max : score > 0 ? score : 1;
    return { score, max, safeMax };
  }

  function extractSkills(section: any): SkillScore[] {
    if (!section || typeof section !== "object") return [];
    return Object.entries(section)
      .filter(
        ([key, value]) =>
          !SUMMARY_KEYS.has(key) && Array.isArray(value) && value.length >= 2,
      )
      .map(([name, value]) => ({
        name,
        label: skillLabel(name),
        score: asNumber(value[0]),
        max: asNumber(value[1]),
      }));
  }

  function modeLabel(key: string): string {
    if (key === "extended") {
      return translate
        ? translate(
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_analysis_extended_label",
          )
        : t(
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_analysis_extended_label",
          );
    }
    return translate
      ? translate(
          "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_analysis_vanilla_label",
        )
      : t(
          "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_analysis_vanilla_label",
        );
  }

  function buildModeSummary(key: string, section: any): ModeSummary | null {
    if (!section || typeof section !== "object") return null;
    const { score, max, safeMax } = extractScore(section);
    return {
      key,
      label: modeLabel(key),
      score,
      max,
      safeMax,
      average:
        typeof section.average_points === "number" &&
        Number.isFinite(section.average_points)
          ? section.average_points
          : null,
      competence: typeof section.competence === "string" ? section.competence : "",
      totalBlocks:
        typeof section.total_blocks === "number" &&
        Number.isFinite(section.total_blocks)
          ? section.total_blocks
          : null,
      skills: extractSkills(section),
    };
  }

  $: modeSummaries = (() => {
    translate;
    return ["extended", "vanilla"]
      .map((key) => buildModeSummary(key, analysis?.[key]))
      .filter(Boolean) as ModeSummary[];
  })();

  $: badHabits = analysis?.bad_habits ?? null;
  $: deadCode = badHabits?.deadCode ?? null;
  $: duplicateScript = badHabits?.duplicateScript ?? null;
  $: spriteNaming = badHabits?.spriteNaming ?? null;
  $: backdropNaming = badHabits?.backdropNaming ?? null;

  $: spriteNames =
    Array.isArray(spriteNaming?.sprite) ? spriteNaming.sprite : [];
  $: backdropNames =
    Array.isArray(backdropNaming?.backdrop) ? backdropNaming.backdrop : [];

  function normalizeBlockLines(blocks: any): string[] {
    if (blocks == null) return [];
    if (Array.isArray(blocks)) {
      return blocks.flatMap((entry) => normalizeBlockLines(entry));
    }
    if (typeof blocks === "string") {
      return blocks
        .split("\n")
        .map((line) => line.trim())
        .filter(Boolean);
    }
    return [String(blocks)];
  }

  function normalizeDeadCodeSprites(scripts: any): DeadCodeSprite[] {
    if (!scripts || typeof scripts !== "object") return [];
    return Object.entries(scripts)
      .map(([sprite, scriptMap]) => {
        const parsedScripts = Object.entries(scriptMap ?? {})
          .map(([id, blocks]) => ({
            id,
            blocks: normalizeBlockLines(blocks),
          }))
          .filter((script) => script.blocks.length > 0);
        return { sprite, scripts: parsedScripts };
      })
      .filter((sprite) => sprite.scripts.length > 0);
  }

  $: deadCodeSprites = normalizeDeadCodeSprites(deadCode?.scripts);

  function splitDuplicateGroup(text: any): string[][] {
    if (typeof text !== "string") return [];
    return text
      .split(/\n{2,}/)
      .map((chunk) => normalizeBlockLines(chunk))
      .filter((lines) => lines.length > 0);
  }

  function normalizeDuplicateScripts(scripts: any): DuplicateGroup[] {
    if (!Array.isArray(scripts)) return [];
    return scripts
      .map((groupText) => splitDuplicateGroup(groupText))
      .filter((group) => group.length > 0)
      .map((group) => ({ scripts: group }));
  }

  $: duplicateGroups = normalizeDuplicateScripts(duplicateScript?.scripts);

  function habitToneClass(tone: string): string {
    switch (tone) {
      case "warning":
        return "bg-warning/10 border-warning/20 text-warning";
      case "info":
        return "bg-info/10 border-info/20 text-info";
      case "secondary":
        return "bg-secondary/10 border-secondary/20 text-secondary";
      case "accent":
        return "bg-accent/10 border-accent/20 text-accent";
      default:
        return "bg-base-200/50 border-base-300/50 text-base-content/70";
    }
  }

  $: badHabitCards = badHabits
    ? [
        {
          key: "deadCode",
          labelKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_bad_habit_dead_code",
          count: asNumber(deadCode?.number),
          tone: "warning",
        },
        {
          key: "duplicateScript",
          labelKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_bad_habit_duplicate_script",
          count: asNumber(duplicateScript?.number),
          tone: "info",
        },
        {
          key: "spriteNaming",
          labelKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_bad_habit_sprite_naming",
          count: asNumber(spriteNaming?.number),
          tone: "secondary",
        },
        {
          key: "backdropNaming",
          labelKey:
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_bad_habit_backdrop_naming",
          count: asNumber(backdropNaming?.number),
          tone: "accent",
        },
      ]
    : [];

  $: hasBadHabits = badHabitCards.some((card) => card.count > 0);

  function formatAverage(value: number | null): string {
    if (value == null || !Number.isFinite(value)) return "-";
    return value.toFixed(2);
  }

  function formatCount(value: number | null): string {
    if (value == null || !Number.isFinite(value)) return "-";
    return String(value);
  }

  function openSkillHelp(skill: SkillScore) {
    activeSkillKey = skill.name;
    if (helpDialog && !helpDialog.open) {
      helpDialog.showModal();
    }
  }
</script>

<div class="bg-base-100 rounded-3xl border border-base-200 shadow-lg shadow-base-300/30 overflow-hidden">
  <div class="px-6 py-4 border-b border-base-200 bg-base-100/50 backdrop-blur-sm">
    <div class="flex flex-wrap items-center gap-4">
      <div class="flex items-center gap-3">
        <div class="p-2 bg-primary/10 text-primary rounded-lg">
          <Sparkles size={18} />
        </div>
        <div>
          <h2 class="text-lg font-black tracking-tight">
            {t("frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_analysis_title")}
          </h2>
          <p class="text-[10px] font-black uppercase tracking-widest opacity-40">
            {t(
              "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_analysis_subtitle",
            )}
          </p>
        </div>
      </div>
    </div>
  </div>

  <div class="p-6 space-y-6">
    {#if error}
      <div class="alert bg-error/10 border-error/20 text-error-content rounded-2xl">
        <AlertCircle size={18} />
        <div class="flex flex-col">
          <span class="font-medium text-sm">
            {t("frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_analysis_error")}
          </span>
          <span class="text-xs opacity-70">{error}</span>
        </div>
      </div>
    {:else if !analysis}
      <div class="bg-base-200/40 rounded-2xl border border-base-300/40 p-6 flex items-center gap-3">
        <AlertTriangle size={18} class="text-warning" />
        <div class="text-sm font-medium opacity-70">
          {t("frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_analysis_empty")}
        </div>
      </div>
    {:else}
      <div class="space-y-6">
        <div class="space-y-4">
          <div class="text-[9px] font-black uppercase tracking-widest opacity-40">
            {t("frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_analysis_overall_title")}
          </div>
          <div class="grid lg:grid-cols-2 gap-4">
            {#each modeSummaries as mode}
              <div class="bg-base-200/40 rounded-2xl border border-base-300/40 p-5 space-y-4">
                <div class="flex items-center justify-between gap-3">
                  <div class="flex items-center gap-2">
                    <div class="p-2 bg-primary/10 text-primary rounded-lg">
                      <Trophy size={16} />
                    </div>
                    <div class="text-sm font-black tracking-tight">{mode.label}</div>
                  </div>
                  {#if mode.competence}
                    <span class="badge bg-primary/10 text-primary border-none font-black text-[8px] uppercase tracking-widest">
                      {mode.competence}
                    </span>
                  {/if}
                </div>
                <div class="flex items-end gap-2">
                  <div class="text-3xl font-black">{mode.score}</div>
                  {#if mode.max > 0}
                    <div class="text-sm font-bold opacity-50">/ {mode.max}</div>
                  {/if}
                </div>
                <progress
                  class="progress progress-primary h-2"
                  value={mode.score}
                  max={mode.safeMax}
                ></progress>
                <div class="grid grid-cols-2 gap-3 text-[11px] font-medium">
                  <div class="space-y-1">
                    <div class="text-[9px] font-black uppercase tracking-widest opacity-40">
                      {t(
                        "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_analysis_average_label",
                      )}
                    </div>
                    <div>{formatAverage(mode.average)}</div>
                  </div>
                  <div class="space-y-1">
                    <div class="text-[9px] font-black uppercase tracking-widest opacity-40">
                      {t(
                        "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_analysis_total_blocks_label",
                      )}
                    </div>
                    <div>{formatCount(mode.totalBlocks)}</div>
                  </div>
                </div>
                {#if mode.skills.length}
                  <div class="grid sm:grid-cols-2 gap-3 pt-2">
                    {#each mode.skills as skill}
                      <div class="bg-base-100/70 rounded-xl border border-base-300/40 p-3 space-y-2">
                        <div class="flex items-center justify-between gap-2">
                          <div class="flex items-center gap-2">
                            <div class="text-[11px] font-semibold">
                              {skill.label}
                            </div>
                            <button
                              class="btn btn-ghost btn-xs h-5 min-h-0 w-5 p-0 rounded-full text-[10px] font-black opacity-60 hover:opacity-100"
                              type="button"
                              on:click={() => openSkillHelp(skill)}
                              aria-label={t(
                                "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_label",
                                { skill: skill.label },
                              )}
                              title={t(
                                "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_label",
                                { skill: skill.label },
                              )}
                            >
                              ?
                            </button>
                          </div>
                          <div class="text-[10px] font-black opacity-60">
                            {skill.score}/{skill.max}
                          </div>
                        </div>
                        <progress
                          class="progress progress-secondary h-1.5"
                          value={skill.score}
                          max={skill.max > 0 ? skill.max : 1}
                        ></progress>
                      </div>
                    {/each}
                  </div>
                {/if}
              </div>
            {/each}
          </div>
        </div>

        <div class="space-y-4">
          <div class="flex flex-wrap items-center justify-between gap-3">
            <div class="text-[9px] font-black uppercase tracking-widest opacity-40">
              {t("frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_bad_habits_title")}
            </div>
            {#if !hasBadHabits}
              <div class="flex items-center gap-2 text-success text-[9px] font-black uppercase tracking-widest">
                <CheckCircle2 size={14} />
                {t(
                  "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_bad_habits_clear",
                )}
              </div>
            {/if}
          </div>
          <div class="grid md:grid-cols-2 xl:grid-cols-4 gap-3">
            {#each badHabitCards as card}
              <div class={`rounded-2xl border p-4 ${habitToneClass(card.tone)}`}>
                <div class="text-[9px] font-black uppercase tracking-widest opacity-70">
                  {t(card.labelKey)}
                </div>
                <div class="text-2xl font-black">{card.count}</div>
              </div>
            {/each}
          </div>

          <div class="grid lg:grid-cols-2 gap-4">
            <div class="bg-base-200/40 rounded-2xl border border-base-300/40 p-5 space-y-4">
              <div class="flex items-center justify-between gap-3">
                <div class="flex items-center gap-2">
                  <div class="p-2 bg-warning/10 text-warning rounded-lg">
                    <AlertTriangle size={16} />
                  </div>
                  <div class="text-sm font-black tracking-tight">
                    {t("frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_dead_code_title")}
                  </div>
                </div>
                <span class="badge bg-warning/10 text-warning border-none font-black text-[8px] uppercase tracking-widest">
                  {asNumber(deadCode?.number)}
                </span>
              </div>
              {#if deadCode?.number > 0 && deadCodeSprites.length}
                <div class="space-y-3">
                  {#each deadCodeSprites as sprite}
                    <details class="collapse bg-base-100/60 border border-base-300/50 rounded-xl">
                      <summary class="collapse-title text-sm font-bold flex items-center justify-between">
                        <span>{sprite.sprite}</span>
                        <span class="text-[9px] font-black uppercase tracking-widest opacity-60">
                          {t(
                            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_script_label",
                          )}
                          {sprite.scripts.length}
                        </span>
                      </summary>
                      <div class="collapse-content space-y-3">
                        <div class="grid md:grid-cols-2 gap-3">
                          {#each sprite.scripts as script}
                            <div class="bg-base-100 rounded-xl border border-base-300/50 p-3 space-y-2">
                              <div class="text-[9px] font-black uppercase tracking-widest opacity-50">
                                {t(
                                  "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_script_label",
                                )}
                                {script.id}
                              </div>
                              <div class="max-h-64 overflow-auto pr-1">
                                <ScratchBlocksSnippet script={script.blocks} />
                              </div>
                            </div>
                          {/each}
                        </div>
                      </div>
                    </details>
                  {/each}
                </div>
              {:else}
                <div class="text-xs font-medium opacity-60">
                  {t("frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_dead_code_empty")}
                </div>
              {/if}
            </div>

            <div class="bg-base-200/40 rounded-2xl border border-base-300/40 p-5 space-y-4">
              <div class="flex items-center justify-between gap-3">
                <div class="flex items-center gap-2">
                  <div class="p-2 bg-info/10 text-info rounded-lg">
                    <FileCode size={16} />
                  </div>
                  <div class="text-sm font-black tracking-tight">
                    {t("frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_duplicate_title")}
                  </div>
                </div>
                <span class="badge bg-info/10 text-info border-none font-black text-[8px] uppercase tracking-widest">
                  {asNumber(duplicateScript?.number)}
                </span>
              </div>
              {#if duplicateScript?.number > 0 && duplicateGroups.length}
                <div class="space-y-3">
                  {#each duplicateGroups as group, groupIndex}
                    <div class="bg-base-100/70 rounded-xl border border-base-300/50 p-4 space-y-3">
                      <div class="flex items-center justify-between gap-3">
                        <div class="text-[10px] font-black uppercase tracking-widest opacity-60">
                          {t(
                            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_duplicate_group_label",
                          )} {groupIndex + 1}
                        </div>
                        <span class="badge bg-base-200 text-base-content/70 border-none font-black text-[8px] uppercase tracking-widest">
                          {group.scripts.length}
                        </span>
                      </div>
                      <div class="grid md:grid-cols-2 gap-3">
                        {#each group.scripts as script, scriptIndex}
                          <div class="bg-base-100 rounded-xl border border-base-300/50 p-3 space-y-2">
                            <div class="text-[9px] font-black uppercase tracking-widest opacity-50">
                              {t(
                                "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_script_label",
                              )}
                              {scriptIndex + 1}
                            </div>
                            <div class="max-h-64 overflow-auto pr-1">
                              <ScratchBlocksSnippet script={script} />
                            </div>
                          </div>
                        {/each}
                      </div>
                    </div>
                  {/each}
                </div>
              {:else}
                <div class="text-xs font-medium opacity-60">
                  {t("frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_duplicate_empty")}
                </div>
              {/if}
            </div>
          </div>

          <div class="grid lg:grid-cols-2 gap-4">
            <div class="bg-base-200/40 rounded-2xl border border-base-300/40 p-5 space-y-4">
              <div class="flex items-center justify-between gap-3">
                <div class="flex items-center gap-2">
                  <div class="p-2 bg-secondary/10 text-secondary rounded-lg">
                    <AlertTriangle size={16} />
                  </div>
                  <div class="text-sm font-black tracking-tight">
                    {t(
                      "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_sprite_naming_title",
                    )}
                  </div>
                </div>
                <span class="badge bg-secondary/10 text-secondary border-none font-black text-[8px] uppercase tracking-widest">
                  {asNumber(spriteNaming?.number)}
                </span>
              </div>
              {#if spriteNames.length}
                <div class="flex flex-wrap gap-2">
                  {#each spriteNames as name}
                    <span class="badge bg-base-100 border-base-300/60 text-xs font-semibold">
                      {name}
                    </span>
                  {/each}
                </div>
              {:else}
                <div class="text-xs font-medium opacity-60">
                  {t(
                    "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_sprite_naming_empty",
                  )}
                </div>
              {/if}
            </div>

            <div class="bg-base-200/40 rounded-2xl border border-base-300/40 p-5 space-y-4">
              <div class="flex items-center justify-between gap-3">
                <div class="flex items-center gap-2">
                  <div class="p-2 bg-accent/10 text-accent rounded-lg">
                    <AlertTriangle size={16} />
                  </div>
                  <div class="text-sm font-black tracking-tight">
                    {t(
                      "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_backdrop_naming_title",
                    )}
                  </div>
                </div>
                <span class="badge bg-accent/10 text-accent border-none font-black text-[8px] uppercase tracking-widest">
                  {asNumber(backdropNaming?.number)}
                </span>
              </div>
              {#if backdropNames.length}
                <div class="flex flex-wrap gap-2">
                  {#each backdropNames as name}
                    <span class="badge bg-base-100 border-base-300/60 text-xs font-semibold">
                      {name}
                    </span>
                  {/each}
                </div>
              {:else}
                <div class="text-xs font-medium opacity-60">
                  {t(
                    "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_backdrop_naming_empty",
                  )}
                </div>
              {/if}
            </div>
          </div>
        </div>
      </div>
    {/if}
  </div>
</div>

<dialog
  bind:this={helpDialog}
  class="modal"
  on:close={() => (activeSkillKey = null)}
>
  <div class="modal-box max-w-2xl p-0 overflow-hidden rounded-3xl border border-base-200 shadow-2xl bg-base-100 max-h-[85vh] flex flex-col">
    <div class="px-6 py-5 border-b border-base-200 flex items-center justify-between gap-4">
      <div class="flex items-center gap-3">
        <div class="w-10 h-10 rounded-2xl bg-primary/10 text-primary flex items-center justify-center text-lg font-black">
          ?
        </div>
        <h3 class="text-lg font-black tracking-tight">
          {t(
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_heading",
            { skill: activeSkillLabel },
          )}
        </h3>
      </div>
      <form method="dialog">
        <button
          class="btn btn-ghost btn-circle btn-sm"
          aria-label={t(
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_close",
          )}
        >
          <X class="size-4" />
        </button>
      </form>
    </div>
    <div class="px-6 py-5 space-y-4 overflow-y-auto flex-1">
      {#if activeHelp}
        <p class="text-sm font-medium text-base-content/80">
          {t(activeHelp.summaryKey)}
        </p>
        {#if activeHelp.bulletsKeys.length}
          <ul class="list-disc pl-5 space-y-2 text-sm text-base-content/70">
            {#each activeHelp.bulletsKeys as bulletKey}
              <li>{t(bulletKey)}</li>
            {/each}
          </ul>
        {/if}
        {#if activeHelp.levels.length}
          <div class="space-y-3">
            <div class="text-[9px] font-black uppercase tracking-widest opacity-50">
              {t(
                "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_points_heading",
              )}
            </div>
            <div class="grid md:grid-cols-2 gap-3">
              {#each activeHelp.levels as level}
                <div class="rounded-2xl border border-base-200 bg-base-200/40 p-4 space-y-2">
                  <div class="text-[9px] font-black uppercase tracking-widest opacity-60">
                    {t(level.titleKey)}
                  </div>
                  <div class="text-sm text-base-content/80">
                    {t(level.bodyKey)}
                  </div>
                  <div class="space-y-2">
                    <div class="text-[9px] font-black uppercase tracking-widest opacity-50">
                      {t(
                        "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_example_heading",
                      )}
                    </div>
                    <div class="rounded-xl border border-base-300/50 bg-base-100/80 p-3 overflow-x-auto">
                      <ScratchBlocksSnippet
                        script={t(level.exampleKey)}
                        scale={0.75}
                      />
                    </div>
                  </div>
                </div>
              {/each}
            </div>
          </div>
        {/if}
      {:else}
        <p class="text-sm opacity-60">
          {t(
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_missing",
          )}
        </p>
      {/if}
    </div>
    <div class="modal-action bg-base-100/80 border-t border-base-200 px-6 py-4 flex justify-end">
      <form method="dialog">
        <button class="btn btn-ghost rounded-xl font-black uppercase tracking-widest text-[10px]">
          {t(
            "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_close",
          )}
        </button>
      </form>
    </div>
  </div>
  <form
    method="dialog"
    class="modal-backdrop bg-base-content/20 backdrop-blur-sm"
  >
    <button
      aria-label={t(
        "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_close",
      )}
    >
      {t(
        "frontend/src/lib/components/ScratchAnalysisPanel.svelte::scratch_skill_help_close",
      )}
    </button>
  </form>
</dialog>
