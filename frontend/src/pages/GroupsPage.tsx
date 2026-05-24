import { useEffect, useState } from "react";
import { getJSON, putJSON } from "../lib/api";

type GroupDef = {
  name: string;
  type: string;
  url?: string;
  interval?: number;
  provider?: string;
};

const typeOptions = [
  { value: "select", label: "select（手动选择）" },
  { value: "url-test", label: "url-test（自动测速，延迟最低）" },
  { value: "load-balance", label: "load-balance（负载均衡）" },
  { value: "fallback", label: "fallback（故障切换）" },
  { value: "relay", label: "relay（链路中继）" },
  { value: "pass", label: "pass（透传）" },
];

const defaultTestURL = "https://www.gstatic.com/generate_204";
const defaultInterval = 300;
const testGroupTypes = new Set(["url-test", "load-balance", "fallback"]);

export function GroupsPage() {
  const [groups, setGroups] = useState<GroupDef[]>([]);
  const [message, setMessage] = useState("");
  const [saving, setSaving] = useState(false);

  const [editingGroup, setEditingGroup] = useState<GroupDef | null>(null);
  const [draftGroup, setDraftGroup] = useState<GroupDef>({ name: "", type: "select" });

  useEffect(() => {
    void loadGroups();
  }, []);

  async function loadGroups() {
    try {
      const cfg = await getJSON<{ groups: GroupDef[] }>("/api/config/groups");
      setGroups(cfg.groups || []);
    } catch {
      setGroups([]);
    }
  }

  async function persistGroups(nextGroups: GroupDef[], successMessage: string) {
    setSaving(true);
    try {
      await putJSON("/api/config/groups", { groups: nextGroups });
      setGroups(nextGroups);
      setMessage(successMessage);
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "保存失败");
    } finally {
      setSaving(false);
    }
  }

  async function addOrUpdateGroup() {
    if (!draftGroup.name.trim()) return;
    const nextGroup = normalizeGroup({ ...draftGroup, name: draftGroup.name.trim() });
    const duplicate = groups.some((g) => g.name === nextGroup.name && g.name !== editingGroup?.name);
    if (duplicate) {
      setMessage(`分组 ${nextGroup.name} 已存在`);
      return;
    }

    let nextGroups: GroupDef[];
    if (editingGroup) {
      nextGroups = groups.map((g) => g.name === editingGroup.name ? nextGroup : g);
    } else {
      nextGroups = [...groups, nextGroup];
    }
    setDraftGroup({ name: "", type: "select" });
    setEditingGroup(null);
    await persistGroups(nextGroups, editingGroup ? "分组已保存。" : "分组已添加。");
  }

  function editGroup(g: GroupDef) {
    setDraftGroup({ ...g });
    setEditingGroup(g);
  }

  async function deleteGroup(name: string) {
    await persistGroups(groups.filter((g) => g.name !== name), "分组已删除。");
  }

  function normalizeGroup(group: GroupDef): GroupDef {
    const next = { ...group };
    if (testGroupTypes.has(next.type)) {
      next.url = next.url || defaultTestURL;
      next.interval = next.interval || defaultInterval;
    }
    return next;
  }

  function updateDraftType(type: string) {
    setDraftGroup((d) => normalizeGroup({ ...d, type }));
  }

  return (
    <div className="page">
      <div className="page-header">
        <h1>代理分组</h1>
      </div>

      <div className="table-container">
        <table className="modern-table">
          <thead>
            <tr>
              <th>名称</th>
              <th>类型</th>
              <th>URL</th>
              <th>间隔</th>
              <th>Provider</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            {groups.length === 0 ? (
              <tr><td colSpan={6}><div className="empty-state">暂无分组，请添加。</div></td></tr>
            ) : null}
            {groups.map((g) => (
              <tr key={g.name}>
                <td><strong>{g.name}</strong></td>
                <td><span className="badge badge-success">{g.type}</span></td>
                <td style={{ fontSize: 12, fontFamily: "monospace" }}>{g.url || "-"}</td>
                <td>{g.interval || "-"}</td>
                <td>{g.provider ? <span className="badge badge-warning">{g.provider}</span> : "-"}</td>
                <td>
                  <div className="btn-group">
                    <button className="btn btn-sm" onClick={() => editGroup(g)} disabled={saving}>修改</button>
                    <button className="btn btn-sm btn-danger" onClick={() => void deleteGroup(g.name)} disabled={saving}>删除</button>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div style={{ height: 16 }} />

      <div className="form-card">
        <div className="card-header"><h2>{editingGroup ? "修改分组" : "添加分组"}</h2></div>
        <div className="form-grid">
          <div className="form-group">
            <label>名称</label>
            <input value={draftGroup.name} onChange={(e) => setDraftGroup((d) => ({ ...d, name: e.target.value }))} placeholder="PROXY" />
          </div>
          <div className="form-group">
            <label>类型</label>
            <select value={draftGroup.type} onChange={(e) => updateDraftType(e.target.value)}>
              {typeOptions.map((t) => <option key={t.value} value={t.value}>{t.label}</option>)}
            </select>
          </div>
          {testGroupTypes.has(draftGroup.type) ? (
            <>
              <div className="form-group">
                <label>测速 URL</label>
                <input value={draftGroup.url ?? defaultTestURL} onChange={(e) => setDraftGroup((d) => ({ ...d, url: e.target.value }))} />
              </div>
              <div className="form-group">
                <label>间隔 (秒)</label>
                <input type="number" value={draftGroup.interval ?? defaultInterval} onChange={(e) => setDraftGroup((d) => ({ ...d, interval: Number(e.target.value) || defaultInterval }))} />
              </div>
            </>
          ) : null}
          <div className="form-group">
            <label>第三方 Provider</label>
            <input value={draftGroup.provider ?? ""} onChange={(e) => setDraftGroup((d) => ({ ...d, provider: e.target.value }))} placeholder="留空则使用节点" />
            <span style={{ fontSize: 12, color: "#94a3b8" }}>填写 provider 名称后，该分组使用 use 引用而非 proxies</span>
          </div>
        </div>
        <div className="form-footer">
          {editingGroup ? <button className="btn" onClick={() => { setEditingGroup(null); setDraftGroup({ name: "", type: "select" }); }}>取消</button> : null}
          <button className="btn btn-primary" onClick={() => void addOrUpdateGroup()} disabled={saving}>
            {saving ? "保存中..." : editingGroup ? "保存修改" : "添加"}
          </button>
        </div>
      </div>

      <div className="message" style={{ marginTop: 16 }}>{message}</div>

      <div className="message" style={{ marginTop: 8 }}>
        说明：定义代理分组后，在用户页面可以为每个用户指定各分组包含哪些节点。设置了 Provider 的分组使用 <code>use</code> 引用第三方订阅。
      </div>
    </div>
  );
}
