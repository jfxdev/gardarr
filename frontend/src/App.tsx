import { Suspense, lazy } from 'react'
import { Loader2 } from 'lucide-react'
import { BrowserRouter as Router, Routes, Route, Navigate, Outlet, useLocation } from 'react-router'
import { AuthProvider } from './contexts/AuthContext'
import { SetupProvider } from './contexts/SetupContext'
import ProtectedRoute from './components/ProtectedRoute'
import { ErrorBoundary } from './components/ErrorBoundary'
import AppLayout from './AppLayout'
import LoginPage from './Login'
import InitialSetupPage from './InitialSetup'
import { Toaster } from './components/ui/sonner'
import { UpdatePrompt } from './components/pwa/UpdatePrompt'
import { InstallPrompt } from './components/pwa/InstallPrompt'

// Authenticated-area pages are lazy-loaded so the initial bundle only pays
// for the pre-auth screens above; each route's code (and its dependencies,
// e.g. Recharts for Dashboard) loads on first visit instead of up front.
const TorrentsPage = lazy(() => import('./Torrents'))
const WorkersPage = lazy(() => import('./Workers'))
const CategoriesPage = lazy(() => import('./Categories'))
const TagsPage = lazy(() => import('./Tags'))
const RssPage = lazy(() => import('./Rss'))
const DashboardPage = lazy(() => import('./Dashboard'))
const HistoryPage = lazy(() => import('./History'))
const ReportsPage = lazy(() => import('./Reports'))
const IntegrationsPage = lazy(() => import('./Integrations'))
const IntegrationWebhookPage = lazy(() => import('./IntegrationWebhook'))
const IntegrationDiscordPage = lazy(() => import('./IntegrationDiscord'))
const SettingsPage = lazy(() => import('./Settings'))
const AboutPage = lazy(() => import('./About'))
const ProfilePage = lazy(() => import('./Profile'))
const UsersPage = lazy(() => import('./Users'))
const SignupPage = lazy(() => import('./Signup'))
const InviteAcceptPage = lazy(() => import('./InviteAccept'))
const ResetPasswordPage = lazy(() => import('./ResetPassword'))
const ForgotPasswordPage = lazy(() => import('./ForgotPassword'))

function RouteFallback() {
  return (
    <div className="flex h-screen w-full items-center justify-center">
      <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
    </div>
  )
}

// A single top-level ErrorBoundary meant any uncaught render error (a bad
// torrent payload, a chart crash, a modal bug) blanked the entire app,
// including the nav/sidebar and the WS connection. Scope a boundary per
// page instead - keyed on pathname so navigating away from a crashed page
// recovers automatically - so one feature crashing doesn't take down the
// shell.
function RouteErrorBoundary() {
  const location = useLocation()
  return (
    <ErrorBoundary key={location.pathname}>
      <Outlet />
    </ErrorBoundary>
  )
}

function App() {
  return (
    <ErrorBoundary>
      <Router>
        <SetupProvider>
          <AuthProvider>
            <Toaster richColors />
            <UpdatePrompt />
            <InstallPrompt />
            <Suspense fallback={<RouteFallback />}>
              <Routes>
                <Route path="/login" element={<LoginPage />} />
                <Route path="/setup" element={<InitialSetupPage />} />
                <Route path="/signup/:token" element={<SignupPage />} />
                <Route path="/invite/:code" element={<InviteAcceptPage />} />
                <Route path="/reset-password" element={<ResetPasswordPage />} />
                <Route path="/forgot-password" element={<ForgotPasswordPage />} />
                <Route
                  path="/"
                  element={
                    <ProtectedRoute>
                      <AppLayout>
                        <RouteErrorBoundary />
                      </AppLayout>
                    </ProtectedRoute>
                  }
                >
                  <Route index element={<DashboardPage />} />
                  <Route path="worker/:worker_uuid" element={<DashboardPage />} />
                  <Route path="worker/:worker_uuid/task/:uuid" element={<DashboardPage />} />
                  <Route path="torrents" element={<TorrentsPage />} />
                  <Route path="workers" element={<WorkersPage />} />
                  <Route path="categories" element={<CategoriesPage />} />
                  <Route path="tags" element={<TagsPage />} />
                  <Route path="rss" element={<RssPage />} />
                  <Route path="history" element={<HistoryPage />} />
                  <Route path="reports" element={<ReportsPage />} />
                  <Route path="integrations" element={<IntegrationsPage />} />
                  <Route path="integrations/webhooks" element={<IntegrationWebhookPage />} />
                  <Route path="integrations/discord" element={<IntegrationDiscordPage />} />
                  <Route path="users" element={<UsersPage />} />
                  <Route path="settings" element={<ProtectedRoute adminOnly><SettingsPage /></ProtectedRoute>} />
                  <Route path="profile" element={<ProfilePage />} />
                  <Route path="about" element={<AboutPage />} />
                </Route>
                {/* Catch-all route for 404 - redirect to home */}
                <Route path="*" element={<Navigate to="/" replace />} />
              </Routes>
            </Suspense>
          </AuthProvider>
        </SetupProvider>
      </Router>
    </ErrorBoundary>
  )
}

export default App
