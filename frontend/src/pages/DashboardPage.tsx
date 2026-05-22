import { useEffect, useState } from "react";
import { getJSON } from "../lib/api";
import type { DashboardSummary } from "../lib/types";

export function DashboardPage() {
  const [summary, setSummary] = useState<DashboardSummary | null>(null);
  const [message, setMessage] = useState("正在读取服务概况...");

  useEffect(() => {
    void getJSON<DashboardSummary>("/api/dashboard/summary")
      .then((next) => {
        setSummary(next);
        setMessage("数据来自实时读取。");
      })
      .catch((error) => {
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
          <span className="stat-label">3x-ui 状态</span>
          <span className={`stat-value ${summary?.xui_status === "connected" ? "success" : "warning"}`}>
            {summary?.xui_status ?? "-"}
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

      <div className="message">{message}</div>
    </div>
  );
}
