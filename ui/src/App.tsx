import { lazy, Suspense } from 'react';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { ThemeProvider, createTheme, CssBaseline, CircularProgress, Box } from '@mui/material';
import { useUserStore } from './store/userStore';
import ErrorBoundary from './components/ErrorBoundary';
import NotificationQueue from './components/NotificationQueue';

// Eagerly load Login page and SetupWizard (needed immediately)
import Login from './pages/Login';
import SetupWizard from './pages/SetupWizard';

// Lazy load all other pages for code splitting
// User Pages
const Dashboard = lazy(() => import('./pages/Dashboard'));
const Sessions = lazy(() => import('./pages/Sessions'));
const SharedSessions = lazy(() => import('./pages/SharedSessions'));
const InvitationAccept = lazy(() => import('./pages/InvitationAccept'));
const EnhancedCatalog = lazy(() => import('./pages/EnhancedCatalog'));
const EnhancedRepositories = lazy(() => import('./pages/EnhancedRepositories'));
const SessionViewer = lazy(() => import('./pages/SessionViewer'));
const PluginCatalog = lazy(() => import('./pages/PluginCatalog'));
const InstalledPlugins = lazy(() => import('./pages/InstalledPlugins'));
const Scheduling = lazy(() => import('./pages/Scheduling'));
const SecuritySettings = lazy(() => import('./pages/SecuritySettings'));

// Admin Pages (loaded only for admin users)
const AdminDashboard = lazy(() => import('./pages/admin/Dashboard'));
const AdminNodes = lazy(() => import('./pages/admin/Nodes'));
const AdminQuotas = lazy(() => import('./pages/admin/Quotas'));
const AdminPlugins = lazy(() => import('./pages/admin/Plugins'));
const Users = lazy(() => import('./pages/admin/Users'));
const UserDetail = lazy(() => import('./pages/admin/UserDetail'));
const CreateUser = lazy(() => import('./pages/admin/CreateUser'));
const Groups = lazy(() => import('./pages/admin/Groups'));
const GroupDetail = lazy(() => import('./pages/admin/GroupDetail'));
const CreateGroup = lazy(() => import('./pages/admin/CreateGroup'));
const Integrations = lazy(() => import('./pages/admin/Integrations'));
const Scaling = lazy(() => import('./pages/admin/Scaling'));
const Compliance = lazy(() => import('./pages/admin/Compliance'));

// Create React Query client
const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      refetchOnWindowFocus: false,
      retry: 1,
      staleTime: 5000,
    },
  },
});

// Create Material-UI theme
const theme = createTheme({
  palette: {
    mode: 'dark',
    primary: {
      main: '#3f51b5',
    },
    secondary: {
      main: '#f50057',
    },
    background: {
      default: '#0a1929',
      paper: '#132f4c',
    },
  },
  typography: {
    fontFamily: '"Inter", "Roboto", "Helvetica", "Arial", sans-serif',
  },
  components: {
    MuiCard: {
      styleOverrides: {
        root: {
          backgroundImage: 'none',
        },
      },
    },
  },
});

// Protected Route wrapper
function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const isAuthenticated = useUserStore((state) => state.isAuthenticated);

  if (!isAuthenticated) {
    return <Navigate to="/login" replace />;
  }

  return <>{children}</>;
}

// Admin Route wrapper
function AdminRoute({ children }: { children: React.ReactNode }) {
  const isAuthenticated = useUserStore((state) => state.isAuthenticated);
  const user = useUserStore((state) => state.user);

  if (!isAuthenticated) {
    return <Navigate to="/login" replace />;
  }

  if (user?.role !== 'admin') {
    return <Navigate to="/" replace />;
  }

  return <>{children}</>;
}

// Loading fallback component for lazy-loaded pages
function PageLoader() {
  return (
    <Box
      sx={{
        display: 'flex',
        justifyContent: 'center',
        alignItems: 'center',
        minHeight: '100vh',
        backgroundColor: 'background.default',
      }}
    >
      <CircularProgress size={60} />
    </Box>
  );
}

function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <ThemeProvider theme={theme}>
        <CssBaseline />
        <ErrorBoundary>
          <BrowserRouter>
            <Suspense fallback={<PageLoader />}>
              <Routes>
            {/* Public routes */}
            <Route path="/login" element={<Login />} />
            <Route path="/setup" element={<SetupWizard />} />

            {/* Protected user routes */}
            <Route
              path="/"
              element={
                <ProtectedRoute>
                  <Dashboard />
                </ProtectedRoute>
              }
            />
            <Route
              path="/sessions"
              element={
                <ProtectedRoute>
                  <Sessions />
                </ProtectedRoute>
              }
            />
            <Route
              path="/sessions/:sessionId/viewer"
              element={
                <ProtectedRoute>
                  <SessionViewer />
                </ProtectedRoute>
              }
            />
            <Route
              path="/shared-sessions"
              element={
                <ProtectedRoute>
                  <SharedSessions />
                </ProtectedRoute>
              }
            />
            <Route
              path="/invite/:token"
              element={
                <ProtectedRoute>
                  <InvitationAccept />
                </ProtectedRoute>
              }
            />
            <Route
              path="/catalog"
              element={
                <ProtectedRoute>
                  <EnhancedCatalog />
                </ProtectedRoute>
              }
            />
            <Route
              path="/repositories"
              element={
                <ProtectedRoute>
                  <EnhancedRepositories />
                </ProtectedRoute>
              }
            />
            <Route
              path="/plugins/catalog"
              element={
                <ProtectedRoute>
                  <PluginCatalog />
                </ProtectedRoute>
              }
            />
            <Route
              path="/plugins/installed"
              element={
                <ProtectedRoute>
                  <InstalledPlugins />
                </ProtectedRoute>
              }
            />
            <Route
              path="/scheduling"
              element={
                <ProtectedRoute>
                  <Scheduling />
                </ProtectedRoute>
              }
            />
            <Route
              path="/security"
              element={
                <ProtectedRoute>
                  <SecuritySettings />
                </ProtectedRoute>
              }
            />
            <Route
              path="/admin"
              element={
                <AdminRoute>
                  <AdminDashboard />
                </AdminRoute>
              }
            />
            <Route
              path="/admin/dashboard"
              element={
                <AdminRoute>
                  <AdminDashboard />
                </AdminRoute>
              }
            />
            <Route
              path="/admin/nodes"
              element={
                <AdminRoute>
                  <AdminNodes />
                </AdminRoute>
              }
            />
            <Route
              path="/admin/quotas"
              element={
                <AdminRoute>
                  <AdminQuotas />
                </AdminRoute>
              }
            />
            <Route
              path="/admin/plugins"
              element={
                <AdminRoute>
                  <AdminPlugins />
                </AdminRoute>
              }
            />
            <Route
              path="/admin/users"
              element={
                <AdminRoute>
                  <Users />
                </AdminRoute>
              }
            />
            <Route
              path="/admin/users/create"
              element={
                <AdminRoute>
                  <CreateUser />
                </AdminRoute>
              }
            />
            <Route
              path="/admin/users/:userId"
              element={
                <AdminRoute>
                  <UserDetail />
                </AdminRoute>
              }
            />
            <Route
              path="/admin/groups"
              element={
                <AdminRoute>
                  <Groups />
                </AdminRoute>
              }
            />
            <Route
              path="/admin/groups/create"
              element={
                <AdminRoute>
                  <CreateGroup />
                </AdminRoute>
              }
            />
            <Route
              path="/admin/groups/:groupId"
              element={
                <AdminRoute>
                  <GroupDetail />
                </AdminRoute>
              }
            />
            <Route
              path="/admin/integrations"
              element={
                <AdminRoute>
                  <Integrations />
                </AdminRoute>
              }
            />
            <Route
              path="/admin/scaling"
              element={
                <AdminRoute>
                  <Scaling />
                </AdminRoute>
              }
            />
            <Route
              path="/admin/compliance"
              element={
                <AdminRoute>
                  <Compliance />
                </AdminRoute>
              }
            />
              </Routes>
            </Suspense>
          </BrowserRouter>
        </ErrorBoundary>

        {/* Global Notification Queue - Production-ready notification system */}
        <NotificationQueue
          maxVisible={3}
          defaultDuration={6000}
          position={{ vertical: 'bottom', horizontal: 'right' }}
          enableHistory={true}
          maxHistorySize={50}
        />
      </ThemeProvider>
    </QueryClientProvider>
  );
}

export default App;
