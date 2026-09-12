import { useEffect, useState } from "react";
import { getJSON, postJSON } from "../lib/api";
import type { ManagedNode } from "../lib/types";

type Server = {
  id: number;
  name: string;
  host: string;
};

const emptyDraft: ManagedNode = {
  id: "",
  name: "",
  address: "",
  port: 443,
  protocol: "vless",
  network: "raw",
  flow: "",
  server_name: "",
};

function supportsServerName(protocol?: string) {
  switch ((protocol || "").toLowerCase()) {
    case "vless":
    case "vmess":
    case "trojan":
      return true;
    default:
      return false;
  }
}

export function NodesPage() {
  const [nodes, setNodes] = useState<ManagedNode[]>([]);
  const [servers, setServers] = useState<Server[]>([]);
  const [draft, setDraft] = useState<ManagedNode>(emptyDraft);
  const [editingID, setEditingID] = useState<string | null>(null);
  const [message, setMessage] = useState("节点代表一台 VPS 机器，你手动添加维护。");
  const [saving, setSaving] = useState(false);
  const [advancedOpen, setAdvancedOpen] = useState(false);
  const [previewUser, setPreviewUser] = useState("");
  const [previewUsers, setPreviewUsers] = useState<{ email: string }[]>([]);
  const [preview, setPreview] = useState("");
  const [previewError, setPreviewError] = useState("");
  const [previewing, setPreviewing] = useState(false);

  useEffect(() => {
    setPreview("");
    setPreviewError("");
  }, [draft, previewUser]);

  useEffect(() => {
    if (advancedOpen) {
      void getJSON<{ email: string }[]>("/api/users/search").then(setPreviewUsers).catch(() => setPreviewUsers([]));
    }
  }, [advancedOpen]);

  async function handlePreview() {
    setPreviewing(true);
    setPreview("");
    setPreviewError("");
    try {
      const result = await postJSON<{ yaml: string }>("/api/nodes/preview", { user: previewUser, node: draft });
      setPreview(result.yaml);
    } catch (error) {
      setPreviewError(error instanceof Error ? error.message : "预览失败");
    } finally {
      setPreviewing(false);
    }
  }

  useEffect(() => {
    void Promise.all([loadNodes(), loadServers()]);
  }, []);

  async function loadNodes() {
    try {
      const next = await getJSON<ManagedNode[]>("/api/nodes");
      setNodes(next);
      setMessage(`已加载 ${next.length} 个节点。`);
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "加载节点失败");
    }
  }

  async function loadServers() {
    try {
      const data = await getJSON<Server[]>("/api/servers");
      setServers(data);
    } catch {
      // non-critical
    }
  }

  async function saveNodes(nextNodes: ManagedNode[], successMessage: string) {
    setSaving(true);
    try {
      const saved = await postJSON<ManagedNode[]>("/api/nodes", nextNodes);
      setNodes(saved);
      setMessage(successMessage);
      return true;
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "保存节点失败");
      return false;
    } finally {
      setSaving(false);
    }
  }

  function resetDraft() {
    setDraft({ ...emptyDraft });
    setEditingID(null);
    setAdvancedOpen(false);
  }

  function handleDelete(id: string) {
    void saveNodes(nodes.filter((n) => n.id !== id), "节点已删除。");
  }

  function handleEdit(node: ManagedNode) {
    setDraft(node);
    setEditingID(node.id);
    setAdvancedOpen(Boolean(node.mihomo_options));
    setMessage(`正在修改节点：${node.name}`);
  }

  async function handleSaveDraft() {
    if (!draft.name.trim() || !draft.address.trim()) {
      setMessage("请填写节点名称和地址。");
      return;
    }
    const nextNode: ManagedNode = {
      id: editingID || "",
      name: draft.name.trim(),
      address: draft.address.trim(),
      port: draft.port || 443,
      protocol: draft.protocol || "vless",
      network: draft.network || "raw",
      flow: draft.flow?.trim() || "",
      server_name: draft.server_name?.trim() || "",
      server_id: draft.server_id,
      mihomo_options: draft.mihomo_options || "",
    };
    const nextNodes = editingID ? nodes.map((n) => (n.id === editingID ? nextNode : n)) : [...nodes, nextNode];
    if (await saveNodes(nextNodes, editingID ? "节点已修改。" : "节点已添加。")) {
      resetDraft();
    }
  }

  const serverMap = new Map(servers.map((s) => [s.id, s.name]));

  return (
    <div className="page">
      <div className="page-header">
        <h1>节点</h1>
        <div className="page-actions">
          <button type="button" className="btn" onClick={() => void loadNodes()} disabled={saving}>刷新</button>
        </div>
      </div>

      <div className="table-container">
        <table className="modern-table">
          <thead>
            <tr>
              <th>名称</th>
              <th>地址</th>
              <th>端口</th>
              <th>协议</th>
              <th>网络</th>
              <th>流控</th>
              <th>SNI</th>
              <th>所属服务器</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            {nodes.length === 0 ? (
              <tr><td colSpan={9}><div className="empty-state">暂无节点，请添加。</div></td></tr>
            ) : null}
            {nodes.map((node) => (
              <tr key={node.id}>
                <td><strong>{node.name}</strong>{node.mihomo_options?.trim() ? <span className="node-options-badge">自定义参数</span> : null}</td>
                <td style={{ fontFamily: "monospace" }}>{node.address}</td>
                <td>{node.port || 443}</td>
                <td>{node.protocol || "vless"}</td>
                <td>{node.network || "raw"}</td>
                <td>{node.flow || "-"}</td>
                <td style={{ fontFamily: "monospace" }}>{node.server_name || "-"}</td>
                <td style={{ fontSize: 13, color: "#64748b" }}>{serverMap.get(node.server_id ?? 0) || "-"}</td>
                <td>
                  <div className="btn-group">
                    <button type="button" className="btn btn-sm" disabled={saving || previewing} onClick={() => handleEdit(node)}>修改</button>
                    <button type="button" className="btn btn-sm btn-danger" disabled={saving || previewing} onClick={() => handleDelete(node.id)}>删除</button>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div className="message" style={{ marginTop: 16 }}>
        节点 = VPS 机器。你手动添加，系统不会从 3x-ui 自动发现。用户页勾选节点后，该 VPS 上对应用户的入站会生成配置。
      </div>

      <div style={{ height: 16 }} />

      <div className="form-card">
        <div className="card-header"><h2>{editingID ? "修改节点" : "添加节点"}</h2></div>
        <fieldset disabled={saving || previewing} className="node-editor-fields">
        <div className="form-grid">
          <div className="form-group">
            <label>名称</label>
            <input value={draft.name} onChange={(e) => setDraft((c) => ({ ...c, name: e.target.value }))} placeholder="例如: hk" />
          </div>
          <div className="form-group">
            <label>地址</label>
            <input value={draft.address} onChange={(e) => setDraft((c) => ({ ...c, address: e.target.value }))} placeholder="例如: hk.sample.com" />
          </div>
          <div className="form-group">
            <label>端口</label>
            <input type="number" value={draft.port ? String(draft.port) : ""} placeholder="443" onChange={(e) => setDraft((c) => ({ ...c, port: Number(e.target.value) || 0 }))} />
          </div>
          <div className="form-group">
            <label>协议</label>
            <select value={draft.protocol || "vless"} onChange={(e) => setDraft((c) => ({ ...c, protocol: e.target.value }))}>
              <option value="vless">vless</option>
              <option value="vmess">vmess</option>
              <option value="trojan">trojan</option>
              <option value="shadowsocks">shadowsocks</option>
            </select>
          </div>
          <div className="form-group">
            <label>网络类型</label>
            <select value={draft.network || "raw"} onChange={(e) => setDraft((c) => ({ ...c, network: e.target.value }))}>
              <option value="raw">tcp/raw</option>
              <option value="ws">ws</option>
              <option value="grpc">grpc</option>
              <option value="h2">h2</option>
              <option value="http">http</option>
              <option value="xhttp">xhttp</option>
            </select>
          </div>
          <div className="form-group">
            <label>流控</label>
            <input
              list="flow-options"
              value={draft.flow || ""}
              onChange={(e) => setDraft((c) => ({ ...c, flow: e.target.value }))}
              placeholder="留空"
            />
            <datalist id="flow-options">
              <option value="xtls-rprx-vision" />
              <option value="xtls-rprx-direct" />
            </datalist>
          </div>
          {supportsServerName(draft.protocol) ? (
            <div className="form-group">
              <label>SNI / ServerName</label>
              <input
                value={draft.server_name || ""}
                onChange={(e) => setDraft((c) => ({ ...c, server_name: e.target.value }))}
                placeholder="留空时优先读取面板配置，否则回退到地址"
              />
            </div>
          ) : null}
          <div className="form-group">
            <label>所属服务器</label>
            <select value={draft.server_id ?? ""} onChange={(e) => setDraft((c) => ({ ...c, server_id: e.target.value ? Number(e.target.value) : undefined }))}>
              <option value="">-- 不关联 --</option>
              {servers.map((sv) => (
                <option key={sv.id} value={sv.id}>{sv.name} ({sv.host})</option>
              ))}
            </select>
          </div>
        </div>
        <details className="node-advanced" open={advancedOpen} onToggle={(event) => setAdvancedOpen(event.currentTarget.open)}>
          <summary>高级配置 <span>Mihomo YAML · 可选</span></summary>
          <div className="node-advanced-body">
            <p>在自动生成的节点配置上补充或覆盖参数。嵌套对象保留未填写的字段，数组整体替换；留空使用默认配置。</p>
            <div className="node-options-toolbar">
              <label htmlFor="mihomo-options">自定义参数</label>
              <button type="button" className="btn btn-sm" disabled={Boolean(draft.mihomo_options?.trim())} onClick={() => setDraft((c) => ({ ...c, mihomo_options: "reality-opts:\n  support-x25519mlkem768: true\n" }))}>填入 REALITY 示例</button>
            </div>
            <textarea id="mihomo-options" className="node-yaml-editor" spellCheck={false} value={draft.mihomo_options || ""} onChange={(event) => setDraft((c) => ({ ...c, mihomo_options: event.target.value }))} placeholder={"reality-opts:\n  support-x25519mlkem768: true"} rows={8} />
            <p className="node-options-hint">直接填写单个节点的参数，不需要 proxies: 或列表前缀。节点名称请在上方修改。参数支持情况取决于 Mihomo 版本。</p>
            <div className="node-preview-controls">
              <div className="form-group">
                <label htmlFor="node-preview-user">预览用户</label>
                <select id="node-preview-user" value={previewUser} onChange={(event) => setPreviewUser(event.target.value)}>
                  <option value="">选择用户以读取其入站配置</option>
                  {previewUsers.map((user) => <option key={user.email} value={user.email}>{user.email}</option>)}
                </select>
              </div>
              <button type="button" className="btn" disabled={!previewUser || previewing} onClick={() => void handlePreview()}>{previewing ? "正在生成…" : "预览合并结果"}</button>
            </div>
            <p className="node-options-hint">使用当前未保存的表单与该用户的入站生成单个节点配置；不会保存修改或更改用户的节点选择。</p>
            {previewError ? <div role="alert" className="message">{previewError}</div> : null}
            {preview ? <div className="node-preview-result"><strong>合并后的节点配置</strong><pre>{preview}</pre></div> : null}
          </div>
        </details>
        <div className="form-footer">
          {editingID ? <button type="button" className="btn" onClick={resetDraft}>取消</button> : null}
          <button type="button" className="btn btn-primary" disabled={saving || previewing} onClick={() => void handleSaveDraft()}>
            {editingID ? "保存修改" : "添加节点"}
          </button>
        </div>
        </fieldset>
      </div>

      <div className="message" role="status" style={{ marginTop: 16 }}>{message}</div>
    </div>
  );
}
