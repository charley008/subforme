import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { getJSON, postJSON } from "../lib/api";
import type { AuthStatus } from "../lib/types";

export function LoginPage() {
  const navigate = useNavigate();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [message, setMessage] = useState("请输入后台管理员账号密码。");
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    void getJSON<AuthStatus>("/api/auth/me")
      .then((status) => {
        if (status.authenticated) {
          navigate("/");
        }
      })
      .catch(() => {});
  }, [navigate]);

  async function handleLogin() {
    setLoading(true);
    try {
      await postJSON("/api/auth/login", { username, password });
      navigate("/");
    } catch {
      setMessage("登录失败，请检查账号和密码。");
    } finally {
      setLoading(false);
    }
  }

  return (
    <div style={{
      minHeight: "100vh",
      display: "grid",
      placeItems: "center",
      background: "linear-gradient(135deg, #0f172a 0%, #1e293b 100%)",
      padding: "24px",
    }}>
      <div style={{
        width: "min(400px, 100%)",
        background: "#fff",
        borderRadius: "16px",
        boxShadow: "0 25px 50px -12px rgba(0,0,0,0.5)",
        overflow: "hidden",
      }}>
        <div style={{
          padding: "32px 32px 0",
          textAlign: "center",
        }}>
          <div style={{
            width: "56px",
            height: "56px",
            background: "linear-gradient(135deg, #6366f1, #a78bfa)",
            borderRadius: "14px",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            fontSize: "24px",
            color: "white",
            fontWeight: "bold",
            margin: "0 auto 16px",
          }}>S</div>
          <h1 style={{ fontSize: "22px", fontWeight: 700, color: "#0f172a", margin: "0 0 4px" }}>SubForMe</h1>
          <p style={{ fontSize: "14px", color: "#64748b", margin: "0 0 24px" }}>Mihomo 配置管理系统</p>
        </div>
        <div style={{ padding: "0 32px 32px", display: "flex", flexDirection: "column", gap: "16px" }}>
          <div style={{ display: "flex", flexDirection: "column", gap: "6px" }}>
            <label style={{ fontSize: "13px", fontWeight: 600, color: "#334155" }}>用户名</label>
            <input
              value={username}
              onChange={(event) => setUsername(event.target.value)}
              onKeyDown={(e) => e.key === "Enter" && !loading && void handleLogin()}
              style={{
                padding: "10px 14px",
                border: "1px solid #e2e8f0",
                borderRadius: "8px",
                fontSize: "14px",
                outline: "none",
              }}
              onFocus={(e) => { e.target.style.borderColor = "#6366f1"; e.target.style.boxShadow = "0 0 0 3px #eef2ff"; }}
              onBlur={(e) => { e.target.style.borderColor = "#e2e8f0"; e.target.style.boxShadow = "none"; }}
            />
          </div>
          <div style={{ display: "flex", flexDirection: "column", gap: "6px" }}>
            <label style={{ fontSize: "13px", fontWeight: 600, color: "#334155" }}>密码</label>
            <input
              type="password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              onKeyDown={(e) => e.key === "Enter" && !loading && void handleLogin()}
              style={{
                padding: "10px 14px",
                border: "1px solid #e2e8f0",
                borderRadius: "8px",
                fontSize: "14px",
                outline: "none",
              }}
              onFocus={(e) => { e.target.style.borderColor = "#6366f1"; e.target.style.boxShadow = "0 0 0 3px #eef2ff"; }}
              onBlur={(e) => { e.target.style.borderColor = "#e2e8f0"; e.target.style.boxShadow = "none"; }}
            />
          </div>
          <button
            type="button"
            onClick={() => void handleLogin()}
            disabled={loading}
            style={{
              padding: "10px 16px",
              border: "none",
              borderRadius: "8px",
              background: loading ? "#94a3b8" : "#6366f1",
              color: "white",
              fontSize: "14px",
              fontWeight: 600,
              cursor: loading ? "not-allowed" : "pointer",
              transition: "background 0.15s",
            }}
            onMouseEnter={(e) => { if (!loading) e.currentTarget.style.background = "#4f46e5"; }}
            onMouseLeave={(e) => { if (!loading) e.currentTarget.style.background = "#6366f1"; }}
          >
            {loading ? "登录中..." : "登录"}
          </button>
          <p style={{ fontSize: "13px", color: "#64748b", textAlign: "center", margin: 0 }}>{message}</p>
        </div>
      </div>
    </div>
  );
}
