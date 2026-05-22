import { useEffect, useState } from "react";
import { NavLink, Outlet, useNavigate } from "react-router-dom";
import { getJSON, postJSON } from "../lib/api";
import type { AuthStatus } from "../lib/types";

const navItems = [
  { to: "/", label: "概览", icon: "📊", end: true },
  { to: "/settings", label: "设置", icon: "⚙️" },
  { to: "/users", label: "用户", icon: "👥" },
  { to: "/nodes", label: "节点", icon: "🖥️" },
  { to: "/groups", label: "代理分组", icon: "🔀" },
  { to: "/templates", label: "模板", icon: "📄" },
];

export function Layout() {
  const navigate = useNavigate();
  const [authChecked, setAuthChecked] = useState(false);
  const [version, setVersion] = useState("");

  useEffect(() => {
    void getJSON<AuthStatus>("/api/auth/me")
      .then((status) => {
        if (!status.authenticated) {
          navigate("/login");
          return;
        }
        setAuthChecked(true);
      })
      .catch(() => {
        navigate("/login");
      });
    void getJSON<{ version: string }>("/api/version")
      .then((v) => setVersion(v.version))
      .catch(() => {});
  }, [navigate]);

  async function handleLogout() {
    try {
      await postJSON("/api/auth/logout");
    } finally {
      navigate("/login");
    }
  }

  if (!authChecked) {
    return (
      <div className="loading-screen">
        <div className="loading-box">正在检查登录状态...</div>
      </div>
    );
  }

  return (
    <div className="app-shell">
      <aside className="sidebar">
        <div className="sidebar-header">
          <div className="sidebar-logo">
            <div className="logo-icon">S</div>
            <span>SubForMe</span>
          </div>
        </div>
        <nav className="sidebar-nav">
          {navItems.map((item) => (
            <NavLink key={item.to} to={item.to} end={item.end}>
              <span className="nav-icon">{item.icon}</span>
              <span className="nav-label">{item.label}</span>
            </NavLink>
          ))}
        </nav>
        <div className="sidebar-footer">
          <div style={{ padding: "4px 14px 8px", fontSize: 11, color: "#475569" }}>{version}</div>
          <button className="sidebar-logout" type="button" onClick={() => void handleLogout()}>
            <span className="nav-icon">🚪</span>
            <span className="nav-label">退出</span>
          </button>
        </div>
      </aside>
      <main className="main-content">
        <Outlet />
      </main>
    </div>
  );
}
