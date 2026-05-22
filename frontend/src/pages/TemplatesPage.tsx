import { useEffect, useState } from "react";
import { YamlEditor } from "../components/YamlEditor";
import { getText, putText } from "../lib/api";

type TemplateSection = "base" | "blacklist" | "whitelist" | "providers";

const sections: { key: TemplateSection; label: string; endpoint: string }[] = [
  { key: "base", label: "基础", endpoint: "/api/config/template?section=base" },
  { key: "whitelist", label: "默认直连", endpoint: "/api/config/template?section=whitelist" },
  { key: "blacklist", label: "默认代理", endpoint: "/api/config/template?section=blacklist" },
  { key: "providers", label: "proxy-providers", endpoint: "/api/config/template?section=providers" },
];

export function TemplatesPage() {
  const [activeKey, setActiveKey] = useState<TemplateSection>("base");
  const [values, setValues] = useState<Record<string, string>>({});
  const [message, setMessage] = useState("基础就是完整 config.yaml 去掉动态部分后的公共配置。");
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    void loadAll();
  }, []);

  async function loadAll() {
    try {
      const results = await Promise.all(
        sections.map((s) =>
          getText(s.endpoint).catch(() => `# ${s.label} 加载失败\n`),
        ),
      );
      const map: Record<string, string> = {};
      sections.forEach((s, i) => { map[s.key] = results[i]; });
      setValues(map);
      setMessage("模板已加载。");
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "加载模板失败");
    }
  }

  async function handleSave() {
    setSaving(true);
    try {
      await putText(
        sections.find((s) => s.key === activeKey)!.endpoint,
        values[activeKey] ?? "",
      );
      setMessage(`${sections.find((s) => s.key === activeKey)!.label}已保存。`);
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "保存模板失败");
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="page">
      <div className="page-header">
        <h1>模板</h1>
      </div>

      <div className="tabs">
        {sections.map((s) => (
          <button
            key={s.key}
            type="button"
            className={`tab ${activeKey === s.key ? "active" : ""}`}
            onClick={() => setActiveKey(s.key)}
          >
            {s.label}
          </button>
        ))}
      </div>

      <div className="card">
        <div className="card-header">
          <h2>{sections.find((s) => s.key === activeKey)?.label ?? activeKey}</h2>
          <div className="page-actions">
            <button type="button" className="btn" onClick={() => void loadAll()}>重新加载</button>
            <button type="button" className="btn btn-primary" onClick={() => void handleSave()} disabled={saving}>
              {saving ? "保存中..." : "保存"}
            </button>
          </div>
        </div>
        <div className="card-body" style={{ padding: 0 }}>
          <YamlEditor
            value={values[activeKey] ?? "# 加载中..."}
            onChange={(value) => setValues((current) => ({ ...current, [activeKey]: value }))}
          />
        </div>
      </div>

      <div className="message" style={{ marginTop: 16 }}>
        说明：基础是完整 config.yaml 去掉动态部分后的公共配置。<code>proxies: []</code>、<code>proxy-groups: []</code>、<code>proxy-providers: {}</code> 是占位符，由系统自动填充，删除后也会重新生成。
      </div>

      <div className="message" style={{ marginTop: 8 }}>{message}</div>
    </div>
  );
}
