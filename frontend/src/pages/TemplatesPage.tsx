import { useEffect, useState } from "react";
import { YamlEditor } from "../components/YamlEditor";
import { getText, putText } from "../lib/api";

type TemplateSection = "base" | "blacklist" | "whitelist" | "ios";

const sections: { key: TemplateSection; label: string; endpoint: string }[] = [
  { key: "base", label: "基础", endpoint: "/api/config/template?section=base" },
  { key: "whitelist", label: "默认直连", endpoint: "/api/config/template?section=whitelist" },
  { key: "blacklist", label: "默认代理", endpoint: "/api/config/template?section=blacklist" },
  { key: "ios", label: "iOS / 移动端", endpoint: "/api/config/template?section=ios" },
];

export function TemplatesPage() {
  const [activeKey, setActiveKey] = useState<TemplateSection>("base");
  const [values, setValues] = useState<Record<string, string>>({});
  const [message, setMessage] = useState("基础是完整 config.yaml 去掉动态部分后的公共配置。");
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    void loadAll();
  }, []);

  async function loadAll() {
    try {
      const results = await Promise.all(
        sections.map((section) =>
          getText(section.endpoint).catch(() => `# ${section.label} 加载失败\n`),
        ),
      );
      const map: Record<string, string> = {};
      sections.forEach((section, index) => {
        map[section.key] = results[index];
      });
      setValues(map);
      setMessage("模板已加载。");
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "加载模板失败");
    }
  }

  async function handleSave() {
    setSaving(true);
    try {
      const section = sections.find((item) => item.key === activeKey)!;
      await putText(section.endpoint, values[activeKey] ?? "");
      setMessage(`${section.label}已保存。`);
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
        {sections.map((section) => (
          <button
            key={section.key}
            type="button"
            className={`tab ${activeKey === section.key ? "active" : ""}`}
            onClick={() => setActiveKey(section.key)}
          >
            {section.label}
          </button>
        ))}
      </div>

      <div className="card">
        <div className="card-header">
          <h2>{sections.find((section) => section.key === activeKey)?.label ?? activeKey}</h2>
          <div className="page-actions">
            <button type="button" className="btn" onClick={() => void loadAll()}>
              重新加载
            </button>
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
        说明：基础模板是公共配置；默认直连、默认代理和 iOS / 移动端模板会与基础模板合并。<code>proxies: []</code> 和{" "}
        <code>proxy-groups: []</code> 是占位符，由系统自动填充。
      </div>

      <div className="message" style={{ marginTop: 8 }}>{message}</div>
    </div>
  );
}
