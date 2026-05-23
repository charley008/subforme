import { useEffect, useState } from "react";
import { getJSON, postJSON, putJSON } from "../lib/api";
import type { AppConfig, AuthStatus } from "../lib/types";

const emptyConfig: AppConfig = {
  mode: "whitelist",
  cache_ttl_seconds: 60,
  healthcheck_url: "https://www.gstatic.com/generate_204",
  healthcheck_interval_seconds: 300,
  xui: {
    base_url: "",
    api_key: "",
    username: "",
    password: "",
  },
};

export function XuiSettingsPage() {
  const [config, setConfig] = useState<AppConfig>(emptyConfig);
  const [message, setMessage] = useState("");
  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState(false);
  const [adminUser, setAdminUser] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [toast, setToast] = useState<string | null>(null);

  useEffect(() => {
    void Promise.all([
      getJSON<AppConfig>("/api/config/app"),
      getJSON<AuthStatus>("/api/auth/me"),
    ]).then(([cfg, auth]) => {
      setConfig(cfg);
      setAdminUser(auth.admin_username ?? "");
    }).catch((error) => {
      setMessage(error instanceof Error ? error.message : "加载设置失败");
    });
  }, []);

  function showToast(msg: string) {
    setToast(msg);
    setTimeout(() => setToast(null), 4000);
  }

  async function handleSave() {
    setSaving(true);
    try {
      await putJSON("/api/config/app", config);
      if (newPassword) {
        await putJSON("/api/auth/password", { password: newPassword });
        showToast(`密码已更新为：${newPassword}`);
        setNewPassword("");
        setMessage("设置已保存，密码已更新。");
      } else {
        setMessage("设置已保存。");
      }
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "保存设置失败");
    } finally {
      setSaving(false);
    }
  }

  async function handleTest() {
    setTesting(true);
    try {
      const next = await postJSON<{ ok: boolean; message?: string }>("/api/xui/test");
      setMessage(next.ok ? "3x-ui 连接成功" : "3x-ui 连接失败: " + (next.message || ""));
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "3x-ui 连通失败");
    } finally {
      setTesting(false);
    }
  }

  return (
    <div className="page">
      <div className="page-header">
        <h1>设置</h1>
        <div className="page-actions">
          <button type="button" className="btn btn-primary" onClick={() => void handleSave()} disabled={saving}>
            {saving ? "保存中..." : "保存"}
          </button>
        </div>
      </div>

      <div className="form-card">
        <div className="card-header">
          <h2>后台管理员</h2>
        </div>
        <div className="form-grid">
          <div className="form-group">
            <label>当前账号</label>
            <input value={adminUser} disabled style={{ background: "#f1f5f9", color: "#475569" }} />
          </div>
          <div className="form-group">
            <label>新密码</label>
            <input
              type="password"
              value={newPassword}
              placeholder="留空则不修改"
              onChange={(e) => setNewPassword(e.target.value)}
            />
            <span style={{ fontSize: 12, color: "#94a3b8" }}>输入新密码后点"保存"即可更新 config.json</span>
          </div>
        </div>
      </div>



      <div className="form-card">
        <div className="card-header"><h2>默认策略</h2></div>
        <div className="form-grid">
          <div className="form-group">
            <label>未匹配规则的流量</label>
            <select value={config.mode} onChange={(e) => setConfig((c) => ({ ...c, mode: e.target.value }))}>
              <option value="whitelist">默认直连 (MATCH,DIRECT)</option>
              <option value="blacklist">默认代理 (MATCH,PROXY)</option>
            </select>
          </div>
          <div className="form-group">
            <label>缓存秒数</label>
            <input type="number" value={String(config.cache_ttl_seconds)} onChange={(e) => setConfig((c) => ({ ...c, cache_ttl_seconds: Number(e.target.value) || 0 }))} />
          </div>
          <div className="form-group">
            <label>健康检查地址</label>
            <input value={config.healthcheck_url} onChange={(e) => setConfig((c) => ({ ...c, healthcheck_url: e.target.value }))} />
          </div>
          <div className="form-group">
            <label>健康检查间隔 (秒)</label>
            <input type="number" value={String(config.healthcheck_interval_seconds)} onChange={(e) => setConfig((c) => ({ ...c, healthcheck_interval_seconds: Number(e.target.value) || 0 }))} />
          </div>
        </div>
      </div>

      <div className="message" style={{ marginTop: 16 }}>{message}</div>

      {toast ? (
        <div style={{
          position: "fixed",
          top: 24,
          right: 24,
          zIndex: 9999,
          background: "#065f46",
          color: "#fff",
          padding: "14px 20px",
          borderRadius: 10,
          boxShadow: "0 10px 25px rgba(0,0,0,0.2)",
          fontSize: 14,
          fontWeight: 500,
          maxWidth: 360,
          wordBreak: "break-all",
          animation: "slideIn 0.3s ease",
        }}>
          {toast}
        </div>
      ) : null}
    </div>
  );
}
