import { useEffect, useMemo, useState } from "react";
import { getJSON, postJSON } from "../lib/api";
import type { DashboardSummary } from "../lib/types";

type BarItem = {
  server_id: number;
  server_name: string;
  total: number;
};

type UserBars = {
  email: string;
  bars: BarItem[];
  total: number;
};

type NodeTrafficTotal = {
  server_id: number;
  server_name: string;
  total: number;
};

type Server = {
  id: number;
  name: string;
  enabled: boolean;
};

type ServerTraffic = {
  server_id: number;
  server_name: string;
  up: number;
  down: number;
};

type ResetResult = {
  server: string;
  reset: number;
  skipped: number;
  errors?: string[];
};

const COLORS = ["#3b82f6", "#ef4444", "#10b981", "#f59e0b", "#8b5cf6", "#ec4899", "#06b6d4", "#f97316"];

function formatBytes(bytes: number): string {
  if (!bytes || bytes === 0) return "0";
  const units = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
  return (bytes / Math.pow(1024, i)).toFixed(i > 0 ? 1 : 0) + units[i];
}

function trafficRows(data: Record<string, ServerTraffic[]>): UserBars[] {
  return Object.entries(data)
    .map(([email, traffic]) => ({
      email,
      bars: traffic.map((t) => ({ server_id: t.server_id, server_name: t.server_name, total: t.up + t.down })),
      total: traffic.reduce((sum, t) => sum + t.up + t.down, 0),
    }))
    .filter((row) => row.total > 0)
    .sort((a, b) => b.total - a.total);
}

export function DashboardPage() {
  const [summary, setSummary] = useState<DashboardSummary | null>(null);
  const [chart, setChart] = useState<UserBars[]>([]);
  const [servers, setServers] = useState<Server[]>([]);
  const [message, setMessage] = useState("\u6b63\u5728\u8bfb\u53d6\u670d\u52a1\u6982\u51b5...");
  const [busy, setBusy] = useState(false);
  const maxVal = chart.length > 0 ? Math.max(...chart.map((u) => u.total)) : 1;
  const visibleServerIDs = useMemo(() => {
    const ids = new Set<number>();
    chart.forEach((u) => u.bars.forEach((b) => ids.add(b.server_id)));
    return ids;
  }, [chart]);
  const nodeTotals = useMemo<NodeTrafficTotal[]>(() => {
    const totals = new Map<number, NodeTrafficTotal>();
    chart.forEach((user) => {
      user.bars.forEach((bar) => {
        const current = totals.get(bar.server_id) ?? {
          server_id: bar.server_id,
          server_name: bar.server_name,
          total: 0,
        };
        current.total += bar.total;
        totals.set(bar.server_id, current);
      });
    });
    return Array.from(totals.values()).filter((item) => item.total > 0).sort((a, b) => b.total - a.total);
  }, [chart]);
  const maxNodeTotal = nodeTotals.length > 0 ? Math.max(...nodeTotals.map((item) => item.total)) : 1;
  const serverColorMap = useMemo(() => {
    const orderedIDs = Array.from(new Set([
      ...servers.map((server) => server.id),
      ...nodeTotals.map((node) => node.server_id),
    ]));
    return new Map(orderedIDs.map((id, index) => [id, COLORS[index % COLORS.length]]));
  }, [nodeTotals, servers]);
  const colorForServer = (serverID: number) => serverColorMap.get(serverID) ?? COLORS[0];

  async function loadDashboard() {
    try {
      const [s, c, serverList] = await Promise.all([
        getJSON<DashboardSummary>("/api/dashboard/summary"),
        getJSON<UserBars[]>("/api/dashboard/traffic").catch(() => [] as UserBars[]),
        getJSON<Server[]>("/api/servers").catch(() => [] as Server[]),
      ]);
      setSummary(s);
      setChart(c);
      setServers(serverList);
      setMessage(`\u5171 ${c.length} \u4e2a\u7528\u6237\u6709\u6d41\u91cf\u6570\u636e`);
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "\u8bfb\u53d6\u6982\u51b5\u5931\u8d25");
    }
  }

  useEffect(() => {
    void loadDashboard();
  }, []);

  async function handleRefreshTraffic() {
    setBusy(true);
    try {
      const data = await postJSON<Record<string, ServerTraffic[]>>("/api/traffic/refresh");
      const rows = trafficRows(data);
      setChart(rows);
      setMessage(`\u5df2\u5237\u65b0 ${rows.length} \u4e2a\u7528\u6237\u7684\u6d41\u91cf\u6570\u636e`);
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "\u5237\u65b0\u6d41\u91cf\u5931\u8d25");
    } finally {
      setBusy(false);
    }
  }

  async function handleResetServer(server: Server) {
    if (!confirm(`\u786e\u5b9a\u91cd\u7f6e ${server.name} \u4e0a\u5df2\u5173\u8054\u7528\u6237\u7684\u6d41\u91cf\uff1f`)) return;
    setBusy(true);
    try {
      const result = await postJSON<ResetResult>(`/api/traffic/reset-server/${server.id}`);
      const errMsg = result.errors?.length ? `, ${result.errors.length} \u4e2a\u5931\u8d25` : "";
      setMessage(`${result.server} \u5df2\u91cd\u7f6e ${result.reset} \u4e2a\u7528\u6237, \u8df3\u8fc7 ${result.skipped} \u4e2a${errMsg}`);
      await handleRefreshTraffic();
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "\u91cd\u7f6e\u6d41\u91cf\u5931\u8d25");
      setBusy(false);
    }
  }

  return (
    <div className="page">
      <div className="page-header">
        <h1>{"\u6982\u89c8"}</h1>
        <div className="page-actions">
          <button type="button" className="btn" onClick={() => void handleRefreshTraffic()} disabled={busy}>
            {"\u5237\u65b0\u6d41\u91cf"}
          </button>
        </div>
      </div>

      <div className="stats-grid">
        <div className="stat-card">
          <span className="stat-label">{"\u670d\u52a1\u72b6\u6001"}</span>
          <span className={`stat-value ${summary?.service === "running" ? "success" : "warning"}`}>
            {summary?.service ?? "-"}
          </span>
        </div>
        <div className="stat-card">
          <span className="stat-label">{"\u670d\u52a1\u5668"}</span>
          <span className="stat-value">
            {summary?.server_count != null ? `${summary.server_count} \u53f0` : "-"}
          </span>
        </div>
        <div className="stat-card">
          <span className="stat-label">{"\u7528\u6237\u6570\u91cf"}</span>
          <span className="stat-value">{summary?.unique_users ?? "-"}</span>
        </div>
        <div className="stat-card">
          <span className="stat-label">{"VPS \u8282\u70b9"}</span>
          <span className="stat-value">{summary?.node_count ?? "-"}</span>
        </div>
        <div className="stat-card">
          <span className="stat-label">{"\u9ed8\u8ba4\u6a21\u5f0f"}</span>
          <span className="stat-value">{summary?.mode ?? "-"}</span>
        </div>
      </div>

      {chart.length > 0 && (
        <div className="card" style={{ marginTop: 20 }}>
          <div className="card-header">
            <h2>节点总流量</h2>
          </div>
          <div className="card-body" style={{ padding: "16px 20px" }}>
            <div style={{ display: "flex", flexDirection: "column", gap: 14 }}>
              {nodeTotals.map((node) => {
                const pct = Math.max((node.total / maxNodeTotal) * 100, 2);
                return (
                  <div key={node.server_id}>
                    <div style={{ display: "flex", justifyContent: "space-between", marginBottom: 4 }}>
                      <strong style={{ fontSize: 13 }}>{node.server_name}</strong>
                      <span style={{ fontSize: 12, color: "var(--gray-500)" }}>{formatBytes(node.total)}</span>
                    </div>
                    <div style={{ height: 18, borderRadius: 6, overflow: "hidden", background: "var(--gray-100)" }}>
                      <div
                        title={`${node.server_name}: ${formatBytes(node.total)}`}
                        style={{
                          width: `${pct}%`,
                          height: "100%",
                          background: colorForServer(node.server_id),
                          borderRadius: 6,
                        }}
                      />
                    </div>
                  </div>
                );
              })}
            </div>
          </div>
        </div>
      )}

      {chart.length > 0 && (
        <div className="card" style={{ marginTop: 20 }}>
          <div className="card-header">
            <h2>{"\u7528\u6237\u6d41\u91cf"}</h2>
            <div className="page-actions">
              {servers.filter((server) => visibleServerIDs.has(server.id)).map((server) => (
                <button key={server.id} type="button" className="btn btn-sm" onClick={() => void handleResetServer(server)} disabled={busy}>
                  {`${server.name} \u91cd\u7f6e\u7528\u6237\u6d41\u91cf`}
                </button>
              ))}
            </div>
          </div>
          <div className="card-body" style={{ padding: "16px 20px" }}>
            <div style={{ display: "flex", flexDirection: "column", gap: 16 }}>
              {chart.map((u) => (
                <div key={u.email}>
                  <div style={{ display: "flex", justifyContent: "space-between", marginBottom: 4 }}>
                    <strong style={{ fontSize: 13 }}>{u.email}</strong>
                    <span style={{ fontSize: 12, color: "var(--gray-500)" }}>{formatBytes(u.total)}</span>
                  </div>
                  <div style={{ display: "flex", height: 24, borderRadius: 6, overflow: "hidden", background: "var(--gray-100)" }}>
                    {u.bars.map((b, i) => {
                      const pct = (b.total / maxVal) * 100;
                      return pct > 0 ? (
                        <div
                          key={b.server_id}
                          title={`${b.server_name}: ${formatBytes(b.total)}`}
                          style={{ width: `${pct}%`, minWidth: 4, background: colorForServer(b.server_id), display: "flex", alignItems: "center", justifyContent: "center", fontSize: 10, color: "#fff", fontWeight: 500 }}
                        >
                          {pct > 15 ? b.server_name : ""}
                        </div>
                      ) : null;
                    })}
                  </div>
                  <div style={{ display: "flex", gap: 12, marginTop: 4, flexWrap: "wrap" }}>
                    {u.bars.map((b, i) => (
                      <span key={b.server_id} style={{ fontSize: 11, color: "var(--gray-500)", display: "flex", alignItems: "center", gap: 3 }}>
                        <span style={{ width: 8, height: 8, borderRadius: 2, background: colorForServer(b.server_id), display: "inline-block" }} />
                        {b.server_name}: {formatBytes(b.total)}
                      </span>
                    ))}
                  </div>
                </div>
              ))}
            </div>
          </div>
        </div>
      )}

      <div className="message" style={{ marginTop: 16 }}>{message}</div>
    </div>
  );
}
