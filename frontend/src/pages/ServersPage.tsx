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
  traffic_sync_interval_minutes: number;
  auto_reset_traffic_enabled: boolean;
  auto_reset_day: number;
  auto_reset_hour: number;
  auto_reset_minute: number;
  auto_reset_timezone: string;
  last_traffic_sync_at: number;
  last_traffic_reset_key: string;
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

const emptyServer: Partial<Server> = {
  scheme: "https",
  port: 2053,
  base_path: "/xui/",
  enabled: true,
  is_main: false,
  traffic_sync_interval_minutes: 60,
  auto_reset_traffic_enabled: false,
  auto_reset_day: 1,
  auto_reset_hour: 0,
  auto_reset_minute: 0,
  auto_reset_timezone: "Asia/Shanghai",
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
      setServers(data.sort((a, b) => Number(b.is_main) - Number(a.is_main)));
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
      const removed = result.removed?.length ? `, ${result.removed.length} 已删除` : "";
      setMessage(`导入完成: ${result.imported} 新建, ${result.updated} 更新, 共 ${result.total} 用户${removed}`);
      await loadServers();
    } catch (e: unknown) {
      setMessage("导入失败: " + (e instanceof Error ? e.message : String(e)));
    }
  }

  async function handleSync() {
    try {
      const result = await postJSON<SyncResult>("/api/sync");
      const errors = result.server_errors?.length
        ? "\n错误: " + result.server_errors.map((e) => `${e.server}: ${e.error}`).join("; ")
        : "";
      setMessage(
        `同步完成: ${result.added} 添加, ${result.updated} 更新, ${result.attached} 关联, ${result.detached} 取消关联, ${result.deleted} 删除${errors}`,
      );
    } catch (e: unknown) {
      setMessage("同步失败: " + (e instanceof Error ? e.message : String(e)));
    }
  }

  return (
    <div className="page">
      <div className="page-header">
        <div>
          <h1>服务器管理</h1>
          <div style={{ marginTop: 4, fontSize: 12, color: "var(--gray-500)" }}>
            目前仅支持 3x-ui，其他面板待开发
          </div>
        </div>
        <div className="page-actions">
          <button className="btn btn-primary" onClick={() => setEditing({ ...emptyServer })}>
            添加服务器
          </button>
          <button className="btn" onClick={() => void handleSync()}>
            同步到非主面板服务器
          </button>
        </div>
      </div>

      {message ? (
        <div className="message info" style={{ marginBottom: 16, whiteSpace: "pre-wrap" }}>
          {message}
          <button style={{ float: "right", background: "none", border: "none", cursor: "pointer" }} onClick={() => setMessage("")}>
            关闭
          </button>
        </div>
      ) : null}

      <div className="table-container">
        <table className="modern-table">
          <thead>
            <tr>
              <th>名称</th>
              <th>地址</th>
              <th>端口</th>
              <th>状态</th>
              <th>主面板</th>
              <th>订阅覆盖</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            {loading ? (
              <tr>
                <td colSpan={7} className="empty-state">
                  加载中...
                </td>
              </tr>
            ) : null}
            {!loading && servers.length === 0 ? (
              <tr>
                <td colSpan={7} className="empty-state">
                  暂无服务器，点击上方按钮添加
                </td>
              </tr>
            ) : null}
            {servers.map((sv) => (
              <tr key={sv.id}>
                <td style={{ fontWeight: 600 }}>{sv.name}</td>
                <td>{sv.host}</td>
                <td>{sv.port}</td>
                <td>
                  <span className={sv.enabled ? "badge badge-success" : "badge badge-warning"}>{sv.enabled ? "启用" : "停用"}</span>
                </td>
                <td>{sv.is_main ? <span className="badge badge-success">主面板</span> : ""}</td>
                <td style={{ color: "var(--gray-500)" }}>{sv.sub_address ? `${sv.sub_address}:${sv.sub_port || sv.port}` : "无"}</td>
                <td>
                  <div className="btn-group">
                    <button className="btn btn-sm" onClick={() => setEditing({ ...sv })}>
                      编辑
                    </button>
                    {sv.is_main ? (
                      <button className="btn btn-sm" onClick={() => void handleImport(sv.id)}>
                        导入
                      </button>
                    ) : null}
                    <button className="btn btn-sm btn-danger" onClick={() => void handleDelete(sv.id)}>
                      删除
                    </button>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {editing ? (
        <div className="form-card" style={{ marginTop: 16 }}>
          <div className="card-header" style={{ padding: "12px 16px" }}>
            <h2 style={{ fontSize: 16 }}>{editing.id ? "编辑服务器" : "添加服务器"}</h2>
          </div>
          <div className="form-grid" style={{ gap: 12, padding: 16 }}>
            <div className="form-group">
              <label>名称 *</label>
              <input placeholder="如 JP-01" value={editing.name || ""} onChange={(e) => setEditing({ ...editing, name: e.target.value })} />
            </div>
            <div className="form-group">
              <label>主机地址 *</label>
              <input placeholder="panel.example.com" value={editing.host || ""} onChange={(e) => setEditing({ ...editing, host: e.target.value })} />
            </div>
            <div className="form-group">
              <label>端口</label>
              <input type="number" value={editing.port || 2053} onChange={(e) => setEditing({ ...editing, port: parseInt(e.target.value, 10) || 2053 })} />
            </div>
            <div className="form-group">
              <label>协议</label>
              <select value={editing.scheme || "https"} onChange={(e) => setEditing({ ...editing, scheme: e.target.value })}>
                <option value="https">https</option>
                <option value="http">http</option>
              </select>
            </div>
            <div className="form-group">
              <label>Base Path</label>
              <input value={editing.base_path || "/xui/"} onChange={(e) => setEditing({ ...editing, base_path: e.target.value })} />
            </div>
            <div className="form-group">
              <label>API Key *</label>
              <div style={{ display: "flex", gap: 8 }}>
                <input
                  type={showKey ? "text" : "password"}
                  placeholder="Bearer token"
                  value={editing.api_key || ""}
                  onChange={(e) => setEditing({ ...editing, api_key: e.target.value })}
                />
                <button type="button" className="btn" onClick={() => setShowKey((v) => !v)}>
                  {showKey ? "隐藏" : "显示"}
                </button>
              </div>
            </div>
            <div className="form-group">
              <label>订阅地址覆盖</label>
              <input placeholder="留空=用主机地址" value={editing.sub_address || ""} onChange={(e) => setEditing({ ...editing, sub_address: e.target.value })} />
            </div>
            <div className="form-group">
              <label>订阅端口覆盖</label>
              <input type="number" placeholder="0=用 inbound 实际端口" value={editing.sub_port || 0} onChange={(e) => setEditing({ ...editing, sub_port: parseInt(e.target.value, 10) || 0 })} />
            </div>
            <div className="form-group">
              <label>备注</label>
              <input value={editing.remark || ""} onChange={(e) => setEditing({ ...editing, remark: e.target.value })} />
            </div>
            <div className="form-group">
              <label>流量刷新间隔（分钟）</label>
              <input
                type="number"
                min={1}
                value={editing.traffic_sync_interval_minutes || 60}
                onChange={(e) => setEditing({ ...editing, traffic_sync_interval_minutes: parseInt(e.target.value, 10) || 60 })}
              />
            </div>
            <div className="form-group">
              <label>清零时区</label>
              <input
                placeholder="Asia/Shanghai"
                value={editing.auto_reset_timezone || "Asia/Shanghai"}
                onChange={(e) => setEditing({ ...editing, auto_reset_timezone: e.target.value })}
              />
            </div>
            <div className="form-group">
              <label>每月清零日期</label>
              <input
                type="number"
                min={1}
                max={31}
                value={editing.auto_reset_day || 1}
                onChange={(e) => setEditing({ ...editing, auto_reset_day: parseInt(e.target.value, 10) || 1 })}
              />
            </div>
            <div className="form-group">
              <label>清零小时</label>
              <input
                type="number"
                min={0}
                max={23}
                value={editing.auto_reset_hour ?? 0}
                onChange={(e) => setEditing({ ...editing, auto_reset_hour: parseInt(e.target.value, 10) || 0 })}
              />
            </div>
            <div className="form-group">
              <label>清零分钟</label>
              <input
                type="number"
                min={0}
                max={59}
                value={editing.auto_reset_minute ?? 0}
                onChange={(e) => setEditing({ ...editing, auto_reset_minute: parseInt(e.target.value, 10) || 0 })}
              />
            </div>
            <div className="form-group" style={{ justifyContent: "end" }}>
              <label style={{ display: "flex", alignItems: "center", gap: 8, minHeight: 38 }}>
                <input
                  type="checkbox"
                  checked={editing.auto_reset_traffic_enabled || false}
                  onChange={(e) => setEditing({ ...editing, auto_reset_traffic_enabled: e.target.checked })}
                />
                启用每月自动清零
              </label>
            </div>
            <div className="form-group" style={{ justifyContent: "end" }}>
              <label style={{ display: "flex", alignItems: "center", gap: 8, minHeight: 38 }}>
                <input type="checkbox" checked={editing.is_main || false} onChange={(e) => setEditing({ ...editing, is_main: e.target.checked })} />
                标记为主面板（导入源）
              </label>
            </div>
          </div>
          <div className="form-footer" style={{ padding: "10px 16px" }}>
            <button className="btn btn-sm" onClick={() => setEditing(null)}>
              取消
            </button>
            <button className="btn btn-sm btn-primary" onClick={() => void handleSave()}>
              保存
            </button>
          </div>
        </div>
      ) : null}
    </div>
  );
}
