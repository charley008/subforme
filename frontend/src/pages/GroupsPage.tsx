import { useEffect, useState } from "react";
import { getJSON, putJSON } from "../lib/api";

type GroupDef = {
  name: string;
  type: string;
  url?: string;
  interval?: number;
  provider?: string;
};

const defaultGroupType = "select";
const defaultTestURL = "https://www.gstatic.com/generate_204";
const defaultInterval = 300;

export function GroupsPage() {
  const [groups, setGroups] = useState<GroupDef[]>([]);
  const [message, setMessage] = useState("");
  const [saving, setSaving] = useState(false);

  const [editingGroup, setEditingGroup] = useState<GroupDef | null>(null);
  const [draftGroup, setDraftGroup] = useState<GroupDef>({
    name: "",
    type: defaultGroupType,
    url: defaultTestURL,
    interval: defaultInterval,
  });

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
    const name = draftGroup.name.trim();
    if (!name) {
      return;
    }

    const nextGroup: GroupDef = {
      ...draftGroup,
      name,
      type: editingGroup?.type || draftGroup.type || defaultGroupType,
      url: draftGroup.url?.trim() || defaultTestURL,
      interval: draftGroup.interval || defaultInterval,
    };

    const duplicate = groups.some((group) => group.name === nextGroup.name && group.name !== editingGroup?.name);
    if (duplicate) {
      setMessage(`分组 ${nextGroup.name} 已存在`);
      return;
    }

    const nextGroups = editingGroup
      ? groups.map((group) => (group.name === editingGroup.name ? nextGroup : group))
      : [...groups, nextGroup];

    resetDraft();
    await persistGroups(nextGroups, editingGroup ? "分组已保存。" : "分组已添加。");
  }

  function editGroup(group: GroupDef) {
    setEditingGroup(group);
    setDraftGroup({
      ...group,
      type: group.type || defaultGroupType,
      url: group.url || defaultTestURL,
      interval: group.interval || defaultInterval,
    });
  }

  async function deleteGroup(name: string) {
    await persistGroups(groups.filter((group) => group.name !== name), "分组已删除。");
  }

  function resetDraft() {
    setEditingGroup(null);
    setDraftGroup({
      name: "",
      type: defaultGroupType,
      url: defaultTestURL,
      interval: defaultInterval,
      provider: "",
    });
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
              <th>URL</th>
              <th>间隔</th>
              <th>Provider</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            {groups.length === 0 ? (
              <tr>
                <td colSpan={5}>
                  <div className="empty-state">暂无分组，请先添加。</div>
                </td>
              </tr>
            ) : null}
            {groups.map((group) => (
              <tr key={group.name}>
                <td>
                  <strong>{group.name}</strong>
                </td>
                <td style={{ fontSize: 12, fontFamily: "monospace" }}>{group.url || "-"}</td>
                <td>{group.interval || "-"}</td>
                <td>{group.provider ? <span className="badge badge-warning">{group.provider}</span> : "-"}</td>
                <td>
                  <div className="btn-group">
                    <button className="btn btn-sm" onClick={() => editGroup(group)} disabled={saving}>
                      修改
                    </button>
                    <button className="btn btn-sm btn-danger" onClick={() => void deleteGroup(group.name)} disabled={saving}>
                      删除
                    </button>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div style={{ height: 16 }} />

      <div className="form-card">
        <div className="card-header">
          <h2>{editingGroup ? "修改分组" : "添加分组"}</h2>
        </div>
        <div className="form-grid">
          <div className="form-group">
            <label>名称</label>
            <input
              value={draftGroup.name}
              onChange={(event) => setDraftGroup((current) => ({ ...current, name: event.target.value }))}
              placeholder="PROXY"
            />
          </div>
          <div className="form-group">
            <label>URL</label>
            <input
              value={draftGroup.url ?? defaultTestURL}
              onChange={(event) => setDraftGroup((current) => ({ ...current, url: event.target.value }))}
              placeholder={defaultTestURL}
            />
          </div>
          <div className="form-group">
            <label>间隔 (秒)</label>
            <input
              type="number"
              value={draftGroup.interval ?? defaultInterval}
              onChange={(event) => setDraftGroup((current) => ({ ...current, interval: Number(event.target.value) || defaultInterval }))}
            />
          </div>
          <div className="form-group">
            <label>第三方 Provider</label>
            <input
              value={draftGroup.provider ?? ""}
              onChange={(event) => setDraftGroup((current) => ({ ...current, provider: event.target.value }))}
              placeholder="留空则使用用户选中的节点"
            />
            <span style={{ fontSize: 12, color: "var(--gray-500)" }}>
              填写 provider 名称后，该分组会使用 <code>use</code> 引用第三方订阅。
            </span>
          </div>
        </div>
        <div className="form-footer">
          {editingGroup ? (
            <button className="btn" onClick={resetDraft}>
              取消
            </button>
          ) : null}
          <button className="btn btn-primary" onClick={() => void addOrUpdateGroup()} disabled={saving}>
            {saving ? "保存中..." : editingGroup ? "保存修改" : "添加"}
          </button>
        </div>
      </div>

      <div className="message" style={{ marginTop: 16 }}>
        {message}
      </div>

      <div className="message" style={{ marginTop: 8 }}>
        说明：这里只定义有哪些代理分组。每个用户具体用哪些节点，以及分组模式是 select、url-test 还是 fallback，都在用户页的“分组设置”里单独配置。
      </div>
    </div>
  );
}
