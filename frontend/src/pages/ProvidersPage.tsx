import { useEffect, useState } from "react";
import { getJSON, postJSON } from "../lib/api";
import type { ProviderAddon } from "../lib/types";

type ProviderRefreshResult = {
  id: string;
  count: number;
  path: string;
  updated_at: number;
};

const emptyDraft: ProviderAddon = {
  id: "",
  source_url: "",
  update_interval_seconds: 3600,
  insecure_skip_verify: true,
};

function formatTime(ts?: number) {
  if (!ts) return "-";
  return new Date(ts * 1000).toLocaleString();
}

export function ProvidersPage() {
  const [providers, setProviders] = useState<ProviderAddon[]>([]);
  const [draft, setDraft] = useState<ProviderAddon>(emptyDraft);
  const [editingID, setEditingID] = useState<string | null>(null);
  const [message, setMessage] = useState("");
  const [busyID, setBusyID] = useState<string | null>(null);

  useEffect(() => {
    void loadProviders();
  }, []);

  async function loadProviders() {
    try {
      const data = await getJSON<ProviderAddon[]>("/api/provider-converters");
      setProviders(data);
      setMessage(`已加载 ${data.length} 个第三方 provider`);
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "加载第三方 providers 失败");
    }
  }

  async function saveProvider(refreshAfterSave = false) {
    if (!draft.id.trim() || !draft.source_url?.trim()) {
      setMessage("请填写 ID 和订阅地址");
      return;
    }
    setBusyID(draft.id);
    try {
      const saved = await postJSON<ProviderAddon>("/api/provider-converters", {
        ...draft,
        id: draft.id.trim(),
        name: draft.id.trim(),
        source_url: draft.source_url.trim(),
        update_interval_seconds: draft.update_interval_seconds || 3600,
      });
      if (refreshAfterSave) {
        const result = await postJSON<ProviderRefreshResult>(`/api/provider-converters/refresh/${encodeURIComponent(saved.id)}`);
        setMessage(`${saved.name} 已保存并刷新，提取 ${result.count} 个节点`);
      } else {
        setMessage(`${saved.name} 已保存`);
      }
      setDraft(emptyDraft);
      setEditingID(null);
      await loadProviders();
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "保存失败");
    } finally {
      setBusyID(null);
    }
  }

  async function refreshProvider(provider: ProviderAddon) {
    setBusyID(provider.id);
    try {
      const result = await postJSON<ProviderRefreshResult>(`/api/provider-converters/refresh/${encodeURIComponent(provider.id)}`);
      setMessage(`${provider.id} 已刷新，提取 ${result.count} 个节点`);
      await loadProviders();
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "刷新失败");
      await loadProviders();
    } finally {
      setBusyID(null);
    }
  }

  async function deleteProvider(provider: ProviderAddon) {
    if (!confirm(`确定删除 ${provider.id} 吗？`)) return;
    setBusyID(provider.id);
    try {
      await postJSON(`/api/provider-converters/delete/${encodeURIComponent(provider.id)}`);
      setMessage(`${provider.id} 已删除`);
      await loadProviders();
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "删除失败");
    } finally {
      setBusyID(null);
    }
  }

  function editProvider(provider: ProviderAddon) {
    setDraft({
      ...provider,
      update_interval_seconds: provider.update_interval_seconds || 3600,
      insecure_skip_verify: provider.insecure_skip_verify ?? false,
    });
    setEditingID(provider.id);
  }

  function resetDraft() {
    setDraft(emptyDraft);
    setEditingID(null);
  }

  return (
    <div className="page">
      <div className="page-header">
        <h1>第三方 Providers</h1>
        <div className="page-actions">
          <button className="btn" onClick={() => void loadProviders()}>刷新列表</button>
        </div>
      </div>

      <div className="table-container">
        <table className="modern-table">
          <thead>
            <tr>
              <th>名称</th>
              <th>节点数</th>
              <th>更新间隔</th>
              <th>最后更新</th>
              <th>状态</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            {providers.length === 0 ? (
              <tr><td colSpan={6}><div className="empty-state">暂无第三方 provider，请在下方添加。</div></td></tr>
            ) : null}
            {providers.map((provider) => (
              <tr key={provider.id}>
                <td><strong>{provider.id}</strong></td>
                <td>{provider.proxy_count ?? "-"}</td>
                <td>{provider.update_interval_seconds || 3600}s</td>
                <td>{formatTime(provider.last_updated_at)}</td>
                <td>
                  {provider.last_error ? (
                    <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
                      <span className="badge badge-danger" title={provider.last_error}>异常</span>
                      <span style={{ maxWidth: 320, color: "#991b1b", fontSize: 12, wordBreak: "break-word" }}>{provider.last_error}</span>
                    </div>
                  ) : (
                    <span className="badge badge-success">正常</span>
                  )}
                </td>
                <td>
                  <div className="btn-group">
                    <button className="btn btn-sm" onClick={() => editProvider(provider)} disabled={busyID === provider.id}>修改</button>
                    <button className="btn btn-sm" onClick={() => void refreshProvider(provider)} disabled={busyID === provider.id}>
                      {busyID === provider.id ? "刷新中..." : "立即刷新"}
                    </button>
                    <button className="btn btn-sm btn-danger" onClick={() => void deleteProvider(provider)} disabled={busyID === provider.id}>删除</button>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div style={{ height: 16 }} />

      <div className="form-card">
        <div className="card-header"><h2>{editingID ? "修改第三方 Provider" : "添加第三方 Provider"}</h2></div>
        <div className="form-grid">
          <div className="form-group">
            <label>名称 / ID</label>
            <input value={draft.id} disabled={Boolean(editingID)} onChange={(e) => setDraft((d) => ({ ...d, id: e.target.value }))} placeholder="例如: provider-main" />
            <span style={{ fontSize: 12, color: "#94a3b8" }}>只能使用字母、数字、下划线和短横线。</span>
          </div>
          <div className="form-group full-width">
            <label>机场订阅地址</label>
            <input value={draft.source_url || ""} onChange={(e) => setDraft((d) => ({ ...d, source_url: e.target.value }))} placeholder="https://example.com/api/v1/client/subscribe?token=..." />
          </div>
          <div className="form-group">
            <label>更新间隔（秒）</label>
            <input type="number" value={draft.update_interval_seconds || 3600} onChange={(e) => setDraft((d) => ({ ...d, update_interval_seconds: Number(e.target.value) || 3600 }))} />
          </div>
          <div className="form-group full-width">
            <label style={{ display: "flex", alignItems: "center", gap: 8, fontWeight: 500 }}>
              <input type="checkbox" checked={draft.insecure_skip_verify || false} onChange={(e) => setDraft((d) => ({ ...d, insecure_skip_verify: e.target.checked }))} />
              跳过订阅源 TLS 证书校验
            </label>
          </div>
        </div>
        <div className="form-footer">
          {editingID ? <button className="btn" onClick={resetDraft}>取消</button> : null}
          <button className="btn" onClick={() => void saveProvider(false)} disabled={Boolean(busyID)}>保存</button>
          <button className="btn btn-primary" onClick={() => void saveProvider(true)} disabled={Boolean(busyID)}>保存并刷新</button>
        </div>
      </div>

      <div className="message" style={{ marginTop: 16 }}>
        系统会把订阅源转换成 <code>config/proxy_providers/ID.yaml</code>，只保留 <code>proxies</code>。用户页勾选该 provider 后，最终订阅会引用 SubForMe 自己的 provider 地址。
      </div>
      <div className="message" style={{ marginTop: 8 }}>{message}</div>
    </div>
  );
}
