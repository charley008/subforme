import { useEffect, useState } from "react";
import { getJSON, putJSON } from "../lib/api";
import type { ManagedNode } from "../lib/types";

const emptyDraft: ManagedNode = {
  id: "",
  name: "",
  address: "",
  port: 443,
};

export function NodesPage() {
  const [nodes, setNodes] = useState<ManagedNode[]>([]);
  const [draft, setDraft] = useState<ManagedNode>(emptyDraft);
  const [editingID, setEditingID] = useState<string | null>(null);
  const [message, setMessage] = useState("节点代表一台 VPS 机器，你手动添加维护。");
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    void loadNodes();
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

  async function saveNodes(nextNodes: ManagedNode[], successMessage: string) {
    setSaving(true);
    try {
      await putJSON("/api/nodes", nextNodes);
      setNodes(nextNodes);
      setMessage(successMessage);
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "保存节点失败");
    } finally {
      setSaving(false);
    }
  }

  function resetDraft() {
    setDraft({ ...emptyDraft });
    setEditingID(null);
  }

  function handleDelete(id: string) {
    void saveNodes(nodes.filter((n) => n.id !== id), "节点已删除。");
  }

  function handleEdit(node: ManagedNode) {
    setDraft(node);
    setEditingID(node.id);
    setMessage(`正在修改节点：${node.name}`);
  }

  function handleSaveDraft() {
    if (!draft.name.trim() || !draft.address.trim()) {
      setMessage("请填写节点名称和地址。");
      return;
    }
    const nextNode: ManagedNode = {
      id: editingID || "",
      name: draft.name.trim(),
      address: draft.address.trim(),
      port: draft.port || 443,
    };
    const nextNodes = editingID ? nodes.map((n) => (n.id === editingID ? nextNode : n)) : [...nodes, nextNode];
    void saveNodes(nextNodes, editingID ? "节点已修改。" : "节点已添加。");
    resetDraft();
  }

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
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            {nodes.length === 0 ? (
              <tr><td colSpan={4}><div className="empty-state">暂无节点，请添加。</div></td></tr>
            ) : null}
            {nodes.map((node) => (
              <tr key={node.id}>
                <td><strong>{node.name}</strong></td>
                <td style={{ fontFamily: "monospace" }}>{node.address}</td>
                <td>{node.port || 443}</td>
                <td>
                  <div className="btn-group">
                    <button type="button" className="btn btn-sm" onClick={() => handleEdit(node)}>修改</button>
                    <button type="button" className="btn btn-sm btn-danger" onClick={() => handleDelete(node.id)}>删除</button>
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
        </div>
        <div className="form-footer">
          {editingID ? <button type="button" className="btn" onClick={resetDraft}>取消</button> : null}
          <button type="button" className="btn btn-primary" onClick={handleSaveDraft}>
            {editingID ? "保存修改" : "添加节点"}
          </button>
        </div>
      </div>

      <div className="message" style={{ marginTop: 16 }}>{message}</div>
    </div>
  );
}
