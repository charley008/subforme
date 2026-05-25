import { useCallback, useEffect, useState } from "react";
import { getJSON, postJSON } from "../lib/api";

type Server = {
  id: number;
  name: string;
  scheme: string;
  host: string;
  port: number;
  base_path: string;
  api_key: string;
  sub_address: string;
  sub_port: number;
  is_main: boolean;
  remark: string;
  enabled: boolean;
  created_at: number;
  updated_at: number;
};

type ImportResult = {
  imported: number;
  updated: number;
  total: number;
  removed?: string[];
};

type SyncResult = {
  synced: number;
  added: number;
  updated: number;
  attached: number;
  detached: number;
  deleted: number;
  server_errors?: { server: string; error: string }[];
};

export function ServersPage() {
  const [servers, setServers] = useState<Server[]>([]);
  const [loading, setLoading] = useState(true);
  const [editing, setEditing] = useState<Partial<Server> | null>(null);
  const [showKey, setShowKey] = useState(false);
  const [message, setMessage] = useState("");

  const loadServers = useCallback(async () => {
    try {
      const data = await getJSON<Server[]>("/api/servers");
      setServers(data.sort((a, b) => (b.is_main ? 1 : 0) - (a.is_main ? 1 : 0)));
    } catch {
      setMessage("加载服务器列表失败");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void loadServers();
  }, [loadServers]);

  async function handleSave() {
    if (!editing) return;
    try {
      if (editing.id) {
        await postJSON(`/api/servers/update/${editing.id}`, editing);
      } else {
        await postJSON("/api/servers/add", editing);
      }
      setEditing(null);
      setMessage("保存成功");
      await loadServers();
    } catch (e: unknown) {
      setMessage("保存失败: " + (e instanceof Error ? e.message : String(e)));
    }
  }

  async function handleDelete(id: number) {
    if (!confirm("确定删除此服务器？")) return;
    try {
      await postJSON(`/api/servers/delete/${id}`);
      setMessage("已删除");
      await loadServers();
    } catch (e: unknown) {
      setMessage("删除失败: " + (e instanceof Error ? e.message : String(e)));
    }
  }

  async function handleImport(id: number) {
    try {
      const result = await postJSON<ImportResult>(`/api/servers/import/${id}`);
      const delMsg = result.removed?.length ? `, ${result.removed.length} 已删除` : "";
      setMessage(`导入完成: ${result.imported} 新建, ${result.updated} 更新, 共 ${result.total} 用户${delMsg}`);
    } catch (e: unknown) {
      setMessage("导入失败: " + (e instanceof Error ? e.message : String(e)));
    }
  }

  async function handleSync() {
    try {
      const result = await postJSON<SyncResult>("/api/sync");
      const errMsg = result.server_errors?.length
        ? "\n错误: " + result.server_errors.map((e) => `${e.server}: ${e.error}`).join("; ")
        : "";
      setMessage(`\u540c\u6b65\u5b8c\u6210: ${result.added} \u6dfb\u52a0, ${result.updated} \u66f4\u65b0, ${result.attached} \u5173\u8054, ${result.detached} \u53d6\u6d88\u5173\u8054, ${result.deleted} \u5220\u9664${errMsg}`);
    } catch (e: unknown) {
      setMessage("同步失败: " + (e instanceof Error ? e.message : String(e)));
    }
  }

  const inputStyle: React.CSSProperties = {
    width: "100%",
    padding: "6px 10px",
    border: "1px solid #d1d5db",
    borderRadius: 6,
    fontSize: 13,
    boxSizing: "border-box",
  };
  const labelStyle: React.CSSProperties = {
    fontSize: 12,
    color: "#64748b",
    marginBottom: 4,
    display: "block",
  };

  return (
    <div style={{ padding: 20 }}>
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 16 }}>
        <div>
          <h2 style={{ margin: 0, fontSize: 20, fontWeight: 700 }}>服务器管理</h2>
          <div style={{ marginTop: 4, fontSize: 12, color: "#64748b" }}>目前仅支持 3x-ui，其他面板待开发</div>
        </div>
        <div style={{ display: "flex", gap: 8 }}>
          <button className="btn btn-sm btn-primary" onClick={() => setEditing({ scheme: "https", port: 2053, base_path: "/xui/", enabled: true })}>
            + 添加服务器
          </button>
          <button className="btn btn-sm btn-default" onClick={() => void handleSync()}>
            同步到非主面板服务器
          </button>
        </div>
      </div>

      {message && (
        <div style={{ padding: "8px 12px", background: "#f0fdf4", border: "1px solid #bbf7d0", borderRadius: 6, marginBottom: 12, fontSize: 13, whiteSpace: "pre-wrap" }}>
          {message}
          <button style={{ float: "right", background: "none", border: "none", cursor: "pointer" }} onClick={() => setMessage("")}>✕</button>
        </div>
      )}

      {loading && <div style={{ textAlign: "center", padding: 40, color: "#94a3b8" }}>加载中...</div>}

      {/* Edit form */}
      {editing && (
        <div style={{ background: "#f8fafc", border: "1px solid #e2e8f0", borderRadius: 8, padding: 16, marginBottom: 16 }}>
          <h3 style={{ margin: "0 0 12px", fontSize: 15 }}>{editing.id ? "编辑服务器" : "添加服务器"}</h3>
          <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 12 }}>
            <div>
              <label style={labelStyle}>名称 *</label>
              <input style={inputStyle} placeholder="如 JP-01" value={editing.name || ""} onChange={(e) => setEditing({ ...editing, name: e.target.value })} />
            </div>
            <div>
              <label style={labelStyle}>主机地址 *</label>
              <input style={inputStyle} placeholder="panel.example.com" value={editing.host || ""} onChange={(e) => setEditing({ ...editing, host: e.target.value })} />
            </div>
            <div>
              <label style={labelStyle}>端口</label>
              <input style={inputStyle} type="number" value={editing.port || 2053} onChange={(e) => setEditing({ ...editing, port: parseInt(e.target.value) || 2053 })} />
            </div>
            <div>
              <label style={labelStyle}>协议</label>
              <select style={inputStyle} value={editing.scheme || "https"} onChange={(e) => setEditing({ ...editing, scheme: e.target.value })}>
                <option value="https">https</option>
                <option value="http">http</option>
              </select>
            </div>
            <div>
              <label style={labelStyle}>Base Path</label>
              <input style={inputStyle} value={editing.base_path || "/xui/"} onChange={(e) => setEditing({ ...editing, base_path: e.target.value })} />
            </div>
            <div>
              <label style={labelStyle}>API Key *</label>
              <div style={{ position: "relative" }}>
                <input style={inputStyle} type={showKey ? "text" : "password"} placeholder="Bearer token" value={editing.api_key || ""} onChange={(e) => setEditing({ ...editing, api_key: e.target.value })} />
                <span onClick={() => setShowKey(!showKey)} style={{ position: "absolute", right: 8, top: "50%", transform: "translateY(-50%)", cursor: "pointer", fontSize: 16, userSelect: "none" }}>
                  {showKey ? "🙈" : "👁️"}
                </span>
              </div>
            </div>
            <div>
              <label style={labelStyle}>订阅地址覆盖（可选，用于 nginx 反代场景）</label>
              <input style={inputStyle} placeholder="留空=用主机地址" value={editing.sub_address || ""} onChange={(e) => setEditing({ ...editing, sub_address: e.target.value })} />
            </div>
            <div>
              <label style={labelStyle}>订阅端口覆盖（可选）</label>
              <input style={inputStyle} type="number" placeholder="0=用 inbound 实际端口" value={editing.sub_port || 0} onChange={(e) => setEditing({ ...editing, sub_port: parseInt(e.target.value) || 0 })} />
            </div>
            <div>
              <label style={labelStyle}>备注</label>
              <input style={inputStyle} value={editing.remark || ""} onChange={(e) => setEditing({ ...editing, remark: e.target.value })} />
            </div>
            <div style={{ display: "flex", alignItems: "center", gap: 8, paddingTop: 20 }}>
              <input type="checkbox" id="is_main" checked={editing.is_main || false} onChange={(e) => setEditing({ ...editing, is_main: e.target.checked })} />
              <label htmlFor="is_main" style={{ fontSize: 13 }}>标记为主面板（导入源）</label>
            </div>
          </div>
          <div style={{ marginTop: 12, display: "flex", gap: 8 }}>
            <button className="btn btn-sm btn-primary" onClick={() => void handleSave()}>保存</button>
            <button className="btn btn-sm btn-default" onClick={() => setEditing(null)}>取消</button>
          </div>
        </div>
      )}

      {/* Server table */}
      <div style={{ overflowX: "auto" }}>
        <table className="data-table" style={{ width: "100%", borderCollapse: "collapse", fontSize: 13 }}>
          <thead>
            <tr style={{ background: "#f1f5f9", textAlign: "left" }}>
              <th style={{ padding: "8px 12px" }}>名称</th>
              <th style={{ padding: "8px 12px" }}>地址</th>
              <th style={{ padding: "8px 12px" }}>端口</th>
              <th style={{ padding: "8px 12px" }}>状态</th>
              <th style={{ padding: "8px 12px" }}>主面板</th>
              <th style={{ padding: "8px 12px" }}>订阅覆盖</th>
              <th style={{ padding: "8px 12px" }}>操作</th>
            </tr>
          </thead>
          <tbody>
            {servers.length === 0 && !loading && (
              <tr><td colSpan={7} style={{ textAlign: "center", padding: 24, color: "#94a3b8" }}>暂无服务器，点击上方按钮添加</td></tr>
            )}
            {servers.map((sv) => (
              <tr key={sv.id} style={{ borderBottom: "1px solid #e2e8f0" }}>
                <td style={{ padding: "8px 12px", fontWeight: 500 }}>{sv.name}</td>
                <td style={{ padding: "8px 12px" }}>{sv.host}</td>
                <td style={{ padding: "8px 12px" }}>{sv.port}</td>
                <td style={{ padding: "8px 12px" }}>
                  <span style={{ color: sv.enabled ? "#16a34a" : "#94a3b8" }}>{sv.enabled ? "启用" : "停用"}</span>
                </td>
                <td style={{ padding: "8px 12px" }}>
                  {sv.is_main ? <span style={{ background: "#dbeafe", color: "#1d4ed8", padding: "2px 8px", borderRadius: 4, fontSize: 12 }}>主面板</span> : ""}
                </td>
                <td style={{ padding: "8px 12px", fontSize: 12, color: "#64748b" }}>
                  {sv.sub_address ? `${sv.sub_address}:${sv.sub_port || sv.port}` : "无"}
                </td>
                <td style={{ padding: "8px 12px" }}>
                  <div style={{ display: "flex", gap: 4 }}>
                    <button className="btn btn-xs" onClick={() => setEditing({ ...sv })}>编辑</button>
                    {sv.is_main && <button className="btn btn-xs" onClick={() => void handleImport(sv.id)}>导入</button>}
                    <button className="btn btn-xs btn-danger" onClick={() => void handleDelete(sv.id)}>删除</button>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

    </div>
  );
}
