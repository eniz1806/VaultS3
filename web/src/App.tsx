import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { AuthProvider } from './hooks/useAuth'
import { ThemeProvider } from './hooks/useTheme'
import ProtectedRoute from './components/ProtectedRoute'
import Layout from './components/Layout'
import LoginPage from './pages/LoginPage'
import BucketsPage from './pages/BucketsPage'
import BucketDetailPage from './pages/BucketDetailPage'
import FileBrowserPage from './pages/FileBrowserPage'
import AccessKeysPage from './pages/AccessKeysPage'
import ActivityPage from './pages/ActivityPage'
import StatsPage from './pages/StatsPage'
import OIDCCallbackPage from './pages/OIDCCallbackPage'

export default function App() {
  return (
    <ThemeProvider>
      <AuthProvider>
        <BrowserRouter basename="/dashboard">
          <Routes>
            <Route path="/login" element={<LoginPage />} />
            <Route path="/oidc-callback" element={<OIDCCallbackPage />} />
            <Route element={<ProtectedRoute />}>
              <Route element={<Layout />}>
                <Route index element={<Navigate to="/buckets" replace />} />
                <Route path="/buckets" element={<BucketsPage />} />
                <Route path="/buckets/:name" element={<BucketDetailPage />} />
                <Route path="/buckets/:name/files" element={<FileBrowserPage />} />
                <Route path="/access-keys" element={<AccessKeysPage />} />
                <Route path="/activity" element={<ActivityPage />} />
                <Route path="/stats" element={<StatsPage />} />
              </Route>
            </Route>
          </Routes>
        </BrowserRouter>
      </AuthProvider>
    </ThemeProvider>
  )
}
