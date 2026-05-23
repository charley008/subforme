import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";
import { Layout } from "./components/Layout";
import { DashboardPage } from "./pages/DashboardPage";
import { GroupsPage } from "./pages/GroupsPage";
import { LoginPage } from "./pages/LoginPage";
import { NodesPage } from "./pages/NodesPage";
import { TemplatesPage } from "./pages/TemplatesPage";
import { ServersPage } from "./pages/ServersPage";
import { UserPreviewPage } from "./pages/UserPreviewPage";
import { XuiSettingsPage } from "./pages/XuiSettingsPage";

const base = (() => {
  const path = window.location.pathname;
  const segments = path.split('/').filter(Boolean);
  // Only treat first segment as basename when there are 2+ segments,
  // meaning we're under a sub-path like /subforme/servers
  // A single segment like /servers is a route, not a deployment base path
  if (segments.length >= 2) {
    return '/' + segments[0];
  }
  return "";
})();

export function App() {
  return (
    <BrowserRouter basename={base}>
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route path="/" element={<Layout />}>
          <Route index element={<DashboardPage />} />
          <Route path="settings" element={<XuiSettingsPage />} />
          <Route path="users" element={<UserPreviewPage />} />
          <Route path="nodes" element={<NodesPage />} />
          <Route path="servers" element={<ServersPage />} />
          <Route path="groups" element={<GroupsPage />} />
          <Route path="templates" element={<TemplatesPage />} />
          <Route path="xui" element={<Navigate to="/settings" replace />} />
          <Route path="preview" element={<Navigate to="/users" replace />} />
        </Route>
      </Routes>
    </BrowserRouter>
  );
}
