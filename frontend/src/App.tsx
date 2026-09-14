import { Suspense, lazy, type ComponentType } from 'react'
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

// Lazy chunk URLs are content-hashed, so they change whenever Vite re-optimizes
// deps in dev or a new build is deployed. A tab that loaded the old index then
// navigates to a route whose chunk hash has since changed gets "Failed to fetch
// dynamically imported module". Retry the import once (covers a transient
// network/optimize blip); if it still fails the chunk is genuinely gone, so
// force a single full reload to pull the current index. sessionStorage guards
// against a reload loop when the failure is not stale-chunk related.
function lazyWithRetry<T extends { default: ComponentType<unknown> }>(factory: () => Promise<T>) {
  return lazy(async () => {
    const reloadKey = 'chunk-reload'
    try {
      const module = await factory()
      window.sessionStorage.removeItem(reloadKey)
      return module
    } catch {
      try {
        return await factory()
      } catch (retryError) {
        if (!window.sessionStorage.getItem(reloadKey)) {
          window.sessionStorage.setItem(reloadKey, '1')
          window.location.reload()
          // Return a never-resolving module so Suspense holds until reload.
          return new Promise<T>(() => {})
        }
        throw retryError
      }
    }
  })
}

// Authenticated-area pages are lazy-loaded so the initial bundle only pays
// for the pre-auth screens above; each route's code (and its dependencies,
// e.g. Recharts for Dashboard) loads on first visit instead of up front.
const TorrentsPage = lazyWithRetry(() => import('./Torrents'))
const WorkersPage = lazyWithRetry(() => import('./Workers'))
const CategoriesPage = lazyWithRetry(() => import('./Categories'))
const TagsPage = lazyWithRetry(() => import('./Tags'))
const RssPage = lazyWithRetry(() => import('./Rss'))
const DashboardPage = lazyWithRetry(() => import('./Dashboard'))
const HistoryPage = lazyWithRetry(() => import('./History'))
const ReportsPage = lazyWithRetry(() => import('./Reports'))
const IntegrationsPage = lazyWithRetry(() => import('./Integrations'))
const IntegrationWebhookPage = lazyWithRetry(() => import('./IntegrationWebhook'))
const IntegrationDiscordPage = lazyWithRetry(() => import('./IntegrationDiscord'))
const SettingsPage = lazyWithRetry(() => import('./Settings'))
const AboutPage = lazyWithRetry(() => import('./About'))
const ProfilePage = lazyWithRetry(() => import('./Profile'))
const UsersPage = lazyWithRetry(() => import('./Users'))
const SignupPage = lazyWithRetry(() => import('./Signup'))
const InviteAcceptPage = lazyWithRetry(() => import('./InviteAccept'))
const ResetPasswordPage = lazyWithRetry(() => import('./ResetPassword'))
const ForgotPasswordPage = lazyWithRetry(() => import('./ForgotPassword'))

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
