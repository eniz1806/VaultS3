import { BrowserRouter, Routes, Route } from 'react-router-dom'
import { AuthProvider } from './hooks/useAuth'
import { ThemeProvider } from './hooks/useTheme'
import { ToastProvider } from './hooks/useToast'
import ProtectedRoute from './components/ProtectedRoute'
import ToastContainer from './components/ToastContainer'
import Layout from './components/Layout'
import LoginPage from './pages/LoginPage'
import HomePage from './pages/HomePage'
import BucketsPage from './pages/BucketsPage'
import BucketDetailPage from './pages/BucketDetailPage'
import FileBrowserPage from './pages/FileBrowserPage'
import AccessKeysPage from './pages/AccessKeysPage'
import ActivityPage from './pages/ActivityPage'
import StatsPage from './pages/StatsPage'
import SettingsPage from './pages/SettingsPage'
import IAMPage from './pages/IAMPage'
import AuditPage from './pages/AuditPage'
import NotificationsPage from './pages/NotificationsPage'
import ReplicationPage from './pages/ReplicationPage'
import LambdaPage from './pages/LambdaPage'
import BackupPage from './pages/BackupPage'
import SearchPage from './pages/SearchPage'
import OIDCCallbackPage from './pages/OIDCCallbackPage'

export default function App() {
  return (
    <ThemeProvider>
      <AuthProvider>
        <ToastProvider>
        <BrowserRouter basename="/dashboard">
          <Routes>
            <Route path="/login" element={<LoginPage />} />
            <Route path="/oidc-callback" element={<OIDCCallbackPage />} />
            <Route element={<ProtectedRoute />}>
              <Route element={<Layout />}>
                <Route index element={<HomePage />} />
                <Route path="/buckets" element={<BucketsPage />} />
                <Route path="/buckets/:name" element={<BucketDetailPage />} />
                <Route path="/buckets/:name/files" element={<FileBrowserPage />} />
                <Route path="/access-keys" element={<AccessKeysPage />} />
                <Route path="/activity" element={<ActivityPage />} />
                <Route path="/stats" element={<StatsPage />} />
                <Route path="/settings" element={<SettingsPage />} />
                <Route path="/iam" element={<IAMPage />} />
                <Route path="/audit" element={<AuditPage />} />
                <Route path="/notifications" element={<NotificationsPage />} />
                <Route path="/replication" element={<ReplicationPage />} />
                <Route path="/lambda" element={<LambdaPage />} />
                <Route path="/backup" element={<BackupPage />} />
                <Route path="/search" element={<SearchPage />} />
              </Route>
            </Route>
          </Routes>
        </BrowserRouter>
        <ToastContainer />
        </ToastProvider>
      </AuthProvider>
    </ThemeProvider>
  )
}
