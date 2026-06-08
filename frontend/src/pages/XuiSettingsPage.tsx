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
  const [restoring, setRestoring] = useState(false);

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

  async function handleBackupExport() {
    try {
      const response = await fetch("/api/backup/export", { credentials: "include" });
      if (!response.ok) {
        throw new Error(await response.text());
      }
      const blob = await response.blob();
      const url = URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = url;
      link.download = `subforme-backup-${new Date().toISOString().slice(0, 19).replace(/[:T]/g, "-")}.zip`;
      document.body.appendChild(link);
      link.click();
      link.remove();
      URL.revokeObjectURL(url);
      setMessage("备份已导出。");
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "导出备份失败");
    }
  }

  async function handleBackupRestore(file: File | null) {
    if (!file) {
      return;
    }
    if (!confirm("恢复会在下次重启 SubForMe 时覆盖当前配置和数据库，确定继续吗？")) {
      return;
    }
    setRestoring(true);
    try {
      const form = new FormData();
      form.append("backup", file);
      const response = await fetch("/api/backup/restore", {
        method: "POST",
        credentials: "include",
        body: form,
      });
      if (!response.ok) {
        throw new Error(await response.text());
      }
      const result = await response.json() as { message?: string };
      setMessage(result.message || "备份已上传，SubForMe 正在自动重启并恢复数据。");
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "恢复备份失败");
    } finally {
      setRestoring(false);
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
            <input value={adminUser} disabled />
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

      <div className="form-card" style={{ marginTop: 16 }}>
        <div className="card-header"><h2>备份与恢复</h2></div>
        <div className="form-grid">
          <div className="form-group">
            <label>导出备份</label>
            <button type="button" className="btn" onClick={() => void handleBackupExport()}>
              下载备份包
            </button>
            <span style={{ fontSize: 12, color: "#94a3b8" }}>包含配置文件、模板、数据库和 provider 文件。</span>
          </div>
          <div className="form-group">
            <label>恢复备份</label>
            <input
              type="file"
              accept=".zip,application/zip"
              disabled={restoring}
              onChange={(event) => void handleBackupRestore(event.target.files?.[0] || null)}
            />
            <span style={{ fontSize: 12, color: "#94a3b8" }}>上传后会自动重启，启动时恢复备份内容。</span>
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
