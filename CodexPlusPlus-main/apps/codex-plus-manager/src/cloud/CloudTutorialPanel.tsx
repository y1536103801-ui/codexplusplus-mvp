import { BookOpen, CheckCircle2, Copy, RotateCcw } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import type { CloudRuntimeState } from "./types";

const storageKey = "codex-plus-cloud-tutorial-dismissed";

type TutorialTemplateId = "fix-bug" | "add-feature" | "explain-code";

type TutorialTemplate = {
  id: TutorialTemplateId;
  title: string;
  description: string;
  prompt: string;
};

type RemoteTutorialTemplate = {
  id?: string;
  title?: string;
  description?: string;
  prompt?: string;
};

type RemoteTutorialContent = {
  title?: string;
  description?: string;
  templates?: RemoteTutorialTemplate[];
  projectDirectoryTips?: string[];
  resultReviewTips?: string[];
  safetyTips?: string[];
};

type Props = {
  state: CloudRuntimeState | null;
  onLaunch: () => Promise<unknown>;
  onRefreshUsage: () => Promise<unknown>;
  onRepairProvider: () => Promise<unknown>;
};

const fallbackTemplates: TutorialTemplate[] = [
  {
    id: "fix-bug",
    title: "修 bug",
    description: "适合有复现步骤、报错或异常行为时使用。",
    prompt: `请在当前项目中帮我修一个 bug。

背景：
- 现象：（这里描述你看到的问题）
- 复现步骤：（这里列出 1、2、3）
- 期望结果：（这里描述正确行为）
- 实际结果：（这里描述当前行为）
- 相关页面或文件：（可选）

要求：
- 先阅读相关代码并给出简短计划。
- 只改必要文件，不自动修改无关配置或 Codex 注入脚本。
- 修改后运行合适的测试或检查，并说明结果。
- 不需要任何密钥、密码、验证码或支付信息。`,
  },
  {
    id: "add-feature",
    title: "加功能",
    description: "适合把一个明确的小需求交给 Codex 实现。",
    prompt: `请在当前项目中实现一个小功能。

目标：
- 功能名称：（这里写功能名）
- 入口或页面：（这里写用户从哪里使用）
- 用户流程：（这里写主要步骤）
- 验收标准：（这里写怎样算完成）

限制：
- 先说明实现计划，再开始修改。
- 保持现有代码风格，避免无关重构。
- 不自动修改 Codex 注入脚本，不需要真实账号凭证。
- 完成后列出改动文件、测试或检查结果。`,
  },
  {
    id: "explain-code",
    title: "解释代码",
    description: "适合先了解项目结构、某段逻辑或风险点。",
    prompt: `请解释当前项目里的这段代码或这些文件。

范围：
- 文件或目录：（这里写路径）
- 我想了解：（整体职责 / 数据流 / 状态变化 / 风险点）

要求：
- 先讲整体作用，再讲关键函数和数据流。
- 标出可能的风险、缺失测试或后续可以问的问题。
- 这次只解释，不修改文件。
- 不需要任何密钥、密码、验证码或支付信息。`,
  },
];

const fallbackProjectDirectoryTips = [
  "选择项目根目录，通常是包含 package.json、Cargo.toml、pyproject.toml 或 .git 的目录。",
  "避免选择系统盘、下载目录或包含大量私人文件的上级目录；只给 Codex 当前任务需要看的项目。",
  "如果只是想了解代码，可以先要求只读分析；需要修改时再明确允许它改哪些范围。",
];

const fallbackResultReviewTips = [
  "开始前先看 Codex 的计划，确认它理解的是你的目标和限制。",
  "修改后看文件列表和 diff，重点确认它没有碰不相关文件或注入脚本。",
  "最后看测试命令、测试结果和未能运行的原因；自己确认通过后再继续下一步。",
];

const fallbackSafetyTips = [
  "不要粘贴私密 Key、密码、支付信息、一次性验证码或可登录的真实凭证。",
  "模板只描述目标、现象和验收标准；服务状态、可用模型和用量以当前页面与后台配置为准。",
  "教程面板不会自动修改用户项目；真正的项目改动来自你明确发出的任务。",
];

function getStoredDismissed() {
  try {
    return window.localStorage.getItem(storageKey) === "1";
  } catch {
    return false;
  }
}

function setStoredDismissed(dismissed: boolean) {
  try {
    window.localStorage.setItem(storageKey, dismissed ? "1" : "0");
  } catch {
    // The in-memory React state still controls the current session if storage is blocked.
  }
}

function cleanText(value: unknown) {
  return typeof value === "string" ? value.trim() : "";
}

function sanitizeTutorialText(text: string) {
  return text
    .replace(/\bsk-[A-Za-z0-9_-]{12,}\b/g, "[已移除敏感内容]")
    .replace(/\bAKIA[0-9A-Z]{16}\b/g, "[已移除敏感内容]")
    .replace(/((?:api|access|secret)[_-]?key|password|token|密码|密钥)\s*[:=]\s*\S+/gi, "$1: [不要粘贴敏感信息]");
}

function readRemoteTutorial(state: CloudRuntimeState | null) {
  const data = (state?.bootstrap.data ?? null) as (RemoteTutorialContent & {
    tutorial?: RemoteTutorialContent | null;
    tutorial_copy?: RemoteTutorialContent | null;
    new_user_tutorial?: RemoteTutorialContent | null;
  }) | null;
  if (!data) return null;

  const nestedTutorial = data.tutorial ?? data.tutorial_copy ?? data.new_user_tutorial;
  if (nestedTutorial) return nestedTutorial;

  if (cleanText(data.title) || cleanText(data.description) || Array.isArray(data.templates)) {
    return data;
  }

  return null;
}

function readTutorialAnnouncement(state: CloudRuntimeState | null) {
  const announcements = state?.bootstrap.data?.announcements ?? [];
  const announcement = announcements.find((item) => item.id === "new_user_tutorial" || item.id.startsWith("new_user_tutorial."));
  return cleanText(announcement?.message);
}

function mergeTemplates(remoteTemplates: RemoteTutorialTemplate[] | undefined) {
  const remoteList = Array.isArray(remoteTemplates) ? remoteTemplates : [];

  return fallbackTemplates.map((fallback) => {
    const remote = remoteList.find((item) => item.id === fallback.id);
    const prompt = cleanText(remote?.prompt);

    return {
      ...fallback,
      title: cleanText(remote?.title) || fallback.title,
      description: sanitizeTutorialText(cleanText(remote?.description) || fallback.description),
      prompt: sanitizeTutorialText(prompt || fallback.prompt),
    };
  });
}

function mergeTips(remoteTips: string[] | undefined, fallbackTips: string[]) {
  if (!Array.isArray(remoteTips)) return fallbackTips;

  const cleanRemoteTips = remoteTips.map((tip) => sanitizeTutorialText(cleanText(tip))).filter(Boolean);
  return cleanRemoteTips.length > 0 ? cleanRemoteTips : fallbackTips;
}

export function CloudTutorialPanel({ state, onLaunch, onRefreshUsage, onRepairProvider }: Props) {
  const [dismissed, setDismissed] = useState(getStoredDismissed);
  const [copyState, setCopyState] = useState<{ id: TutorialTemplateId; status: "copied" | "failed" } | null>(null);
  const status = state?.bootstrap.data?.service.status ?? "not_authenticated";
  const providerActive = state?.managedProvider.active === true;
  const remoteTutorial = useMemo(() => readRemoteTutorial(state), [state]);
  const tutorialAnnouncement = useMemo(() => readTutorialAnnouncement(state), [state]);
  const tutorialTitle = cleanText(remoteTutorial?.title) || "第一次使用";
  const tutorialDescription =
    sanitizeTutorialText(cleanText(remoteTutorial?.description) || tutorialAnnouncement) ||
    "先选对项目目录，再用模板把任务说清楚，最后确认计划、改动和测试结果。";
  const templates = useMemo(() => mergeTemplates(remoteTutorial?.templates), [remoteTutorial?.templates]);
  const projectDirectoryTips = useMemo(
    () => mergeTips(remoteTutorial?.projectDirectoryTips, fallbackProjectDirectoryTips),
    [remoteTutorial?.projectDirectoryTips],
  );
  const resultReviewTips = useMemo(() => mergeTips(remoteTutorial?.resultReviewTips, fallbackResultReviewTips), [remoteTutorial?.resultReviewTips]);
  const safetyTips = useMemo(() => mergeTips(remoteTutorial?.safetyTips, fallbackSafetyTips), [remoteTutorial?.safetyTips]);

  useEffect(() => {
    setStoredDismissed(dismissed);
  }, [dismissed]);

  const steps = useMemo(
    () => [
      { title: "登录账户", done: status !== "not_authenticated" },
      { title: "确认权益和设备状态", done: status === "available" || status === "low_balance" || status === "model_unavailable" },
      { title: "准备 Codex", done: providerActive },
      { title: "启动 Codex 后选择项目目录", done: false, action: onLaunch },
      { title: "刷新用量，失败先修复再看诊断", done: false, action: onRefreshUsage },
    ],
    [onLaunch, onRefreshUsage, providerActive, status],
  );

  const copyTemplate = async (template: TutorialTemplate) => {
    try {
      await navigator.clipboard.writeText(sanitizeTutorialText(template.prompt));
      setCopyState({ id: template.id, status: "copied" });
    } catch {
      setCopyState({ id: template.id, status: "failed" });
    }
  };

  if (dismissed) {
    return (
      <Card className="cloud-panel">
        <CardContent className="cloud-tutorial-restore">
          <span>新手教程已隐藏，关闭状态已记住。</span>
          <Button onClick={() => setDismissed(false)} size="sm" variant="outline">
            <RotateCcw className="h-4 w-4" />
            恢复
          </Button>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card className="cloud-panel">
      <CardHeader className="cloud-panel-head">
        <div>
          <CardTitle>{tutorialTitle}</CardTitle>
          <CardDescription>{tutorialDescription}</CardDescription>
        </div>
        <BookOpen className="h-5 w-5 cloud-muted-icon" />
      </CardHeader>
      <CardContent>
        <div className="cloud-tutorial-list">
          {steps.map((step, index) => (
            <button
              className={`cloud-tutorial-step ${step.done ? "done" : ""}`}
              disabled={!step.action}
              key={step.title}
              onClick={() => void step.action?.()}
              type="button"
            >
              <span>{index + 1}</span>
              <strong>{step.title}</strong>
            </button>
          ))}
        </div>

        <div className="cloud-tutorial-list">
          <p className="cloud-muted">任务模板</p>
          {templates.map((template, index) => {
            const copied = copyState?.id === template.id && copyState.status === "copied";
            const failed = copyState?.id === template.id && copyState.status === "failed";

            return (
              <div className="cloud-tutorial-step" key={template.id}>
                <span>{index + 1}</span>
                <div>
                  <strong>{template.title}</strong>
                  <p className="cloud-muted">{template.description}</p>
                  <pre className="cloud-diagnostics">{template.prompt}</pre>
                  <div className="cloud-secondary-actions">
                    <Button onClick={() => void copyTemplate(template)} size="sm" variant="outline">
                      {copied ? <CheckCircle2 className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
                      {copied ? "已复制" : failed ? "复制失败" : "复制模板"}
                    </Button>
                  </div>
                </div>
              </div>
            );
          })}
        </div>

        <div className="cloud-tutorial-list">
          <p className="cloud-muted">选择项目目录</p>
          {projectDirectoryTips.map((tip, index) => (
            <div className="cloud-tutorial-step" key={tip}>
              <span>{index + 1}</span>
              <strong>{tip}</strong>
            </div>
          ))}
        </div>

        <div className="cloud-tutorial-list">
          <p className="cloud-muted">确认结果</p>
          {resultReviewTips.map((tip, index) => (
            <div className="cloud-tutorial-step" key={tip}>
              <span>{index + 1}</span>
              <strong>{tip}</strong>
            </div>
          ))}
        </div>

        <div className="cloud-tutorial-list">
          <p className="cloud-muted">安全提示</p>
          {safetyTips.map((tip, index) => (
            <div className="cloud-tutorial-step" key={tip}>
              <span>{index + 1}</span>
              <strong>{tip}</strong>
            </div>
          ))}
        </div>

        <div className="cloud-secondary-actions">
          <Button onClick={() => void onRepairProvider()} variant="secondary">修复配置</Button>
          <Button onClick={() => setDismissed(true)} variant="ghost">隐藏教学</Button>
        </div>
      </CardContent>
    </Card>
  );
}
