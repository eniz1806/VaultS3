import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { AuthProvider } from './hooks/useAuth'
import ProtectedRoute from './components/ProtectedRoute'
import Layout from './components/Layout'
import LoginPage from './pages/LoginPage'
import BucketsPage from './pages/BucketsPage'
import BucketDetailPage from './pages/BucketDetailPage'

export default function App() {
  return (
    <AuthProvider>
      <BrowserRouter basename="/dashboard">
        <Routes>
          <Route path="/login" element={<LoginPage />} />
          <Route element={<ProtectedRoute />}>
            <Route element={<Layout />}>
              <Route index element={<Navigate to="/buckets" replace />} />
              <Route path="/buckets" element={<BucketsPage />} />
              <Route path="/buckets/:name" element={<BucketDetailPage />} />
            </Route>
          </Route>
        </Routes>
      </BrowserRouter>
    </AuthProvider>
  )
}
