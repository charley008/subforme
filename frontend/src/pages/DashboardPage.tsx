import { useEffect, useState } from "react";
import { getJSON } from "../lib/api";
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

const COLORS = ["#3b82f6", "#ef4444", "#10b981", "#f59e0b", "#8b5cf6", "#ec4899", "#06b6d4", "#f97316"];

function formatBytes(bytes: number): string {
  if (!bytes || bytes === 0) return "0";
  const units = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
  return (bytes / Math.pow(1024, i)).toFixed(i > 0 ? 1 : 0) + units[i];
}

export function DashboardPage() {
  const [summary, setSummary] = useState<DashboardSummary | null>(null);
  const [chart, setChart] = useState<UserBars[]>([]);
  const [message, setMessage] = useState("正在读取服务概况...");
  const maxVal = chart.length > 0 ? Math.max(...chart.map((u) => u.total)) : 1;

  useEffect(() => {
    void Promise.all([
      getJSON<DashboardSummary>("/api/dashboard/summary"),
      getJSON<UserBars[]>("/api/dashboard/traffic").catch(() => [] as UserBars[]),
    ]).then(([s, c]) => {
      setSummary(s);
      setChart(c);
      setMessage(`共 ${c.length} 个用户有流量数据`);
    }).catch((error) => {
      setMessage(error instanceof Error ? error.message : "读取概况失败");
    });
  }, []);

  return (
    <div className="page">
      <div className="page-header">
        <h1>概览</h1>
      </div>

      <div className="stats-grid">
        <div className="stat-card">
          <span className="stat-label">服务状态</span>
          <span className={`stat-value ${summary?.service === "running" ? "success" : "warning"}`}>
            {summary?.service ?? "-"}
          </span>
        </div>
        <div className="stat-card">
          <span className="stat-label">服务器</span>
          <span className="stat-value">
            {summary?.server_count != null ? `${summary.server_count} 台` : "-"}
          </span>
        </div>
        <div className="stat-card">
          <span className="stat-label">用户数量</span>
          <span className="stat-value">{summary?.unique_users ?? "-"}</span>
        </div>
        <div className="stat-card">
          <span className="stat-label">VPS 节点</span>
          <span className="stat-value">{summary?.node_count ?? "-"}</span>
        </div>
        <div className="stat-card">
          <span className="stat-label">默认模式</span>
          <span className="stat-value">{summary?.mode ?? "-"}</span>
        </div>
      </div>

      {chart.length > 0 && (
        <div className="card" style={{ marginTop: 20 }}>
          <div className="card-header">
            <h2>用户流量</h2>
          </div>
          <div className="card-body" style={{ padding: "16px 20px" }}>
            <div style={{ display: "flex", flexDirection: "column", gap: 16 }}>
              {chart.map((u) => (
                <div key={u.email}>
                  <div style={{ display: "flex", justifyContent: "space-between", marginBottom: 4 }}>
                    <strong style={{ fontSize: 13 }}>{u.email}</strong>
                    <span style={{ fontSize: 12, color: "#64748b" }}>{formatBytes(u.total)}</span>
                  </div>
                  <div style={{ display: "flex", height: 24, borderRadius: 6, overflow: "hidden", background: "#f1f5f9" }}>
                    {u.bars.map((b, i) => {
                      const pct = (b.total / maxVal) * 100;
                      return pct > 0 ? (
                        <div
                          key={b.server_id}
                          title={`${b.server_name}: ${formatBytes(b.total)}`}
                          style={{ width: `${pct}%`, minWidth: 4, background: COLORS[i % COLORS.length], display: "flex", alignItems: "center", justifyContent: "center", fontSize: 10, color: "#fff", fontWeight: 500 }}
                        >
                          {pct > 15 ? b.server_name : ""}
                        </div>
                      ) : null;
                    })}
                  </div>
                  <div style={{ display: "flex", gap: 12, marginTop: 4, flexWrap: "wrap" }}>
                    {u.bars.map((b, i) => (
                      <span key={b.server_id} style={{ fontSize: 11, color: "#64748b", display: "flex", alignItems: "center", gap: 3 }}>
                        <span style={{ width: 8, height: 8, borderRadius: 2, background: COLORS[i % COLORS.length], display: "inline-block" }} />
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
