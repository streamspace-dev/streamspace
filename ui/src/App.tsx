import { lazy, Suspense, useState, useMemo, createContext, useContext } from 'react';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { ThemeProvider, createTheme, CssBaseline, CircularProgress, Box } from '@mui/material';
import { useUserStore } from './store/userStore';
import ErrorBoundary from './components/ErrorBoundary';
import NotificationQueue from './components/NotificationQueue';

// Theme mode context
type ThemeMode = 'light' | 'dark';
interface ThemeContextType {
  mode: ThemeMode;
  toggleTheme: () => void;
}

const ThemeContext = createContext<ThemeContextType>({
  mode: 'dark',
  toggleTheme: () => {},
});

export const useThemeMode = () => useContext(ThemeContext);

// Eagerly load Login page and SetupWizard (needed immediately)
import Login from './pages/Login';
import SetupWizard from './pages/SetupWizard';

// Lazy load all other pages for code splitting
// User Pages
const Dashboard = lazy(() => import('./pages/Dashboard'));
const Sessions = lazy(() => import('./pages/Sessions'));
const SharedSessions = lazy(() => import('./pages/SharedSessions'));
const InvitationAccept = lazy(() => import('./pages/InvitationAccept'));
const SessionViewer = lazy(() => import('./pages/SessionViewer'));
const UserSettings = lazy(() => import('./pages/UserSettings'));

// Admin Content Management Pages (moved from user pages)
const Applications = lazy(() => import('./pages/Applications'));
const Scheduling = lazy(() => import('./pages/Scheduling'));
const SecuritySettings = lazy(() => import('./pages/SecuritySettings'));
const EnhancedRepositories = lazy(() => import('./pages/EnhancedRepositories'));
const PluginCatalog = lazy(() => import('./pages/PluginCatalog'));
const InstalledPlugins = lazy(() => import('./pages/InstalledPlugins'));

// Admin Pages (loaded only for admin users)
const AdminDashboard = lazy(() => import('./pages/admin/Dashboard'));
const AdminNodes = lazy(() => import('./pages/admin/Nodes'));
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
const AuditLogs = lazy(() => import('./pages/admin/AuditLogs'));
const Settings = lazy(() => import('./pages/admin/Settings'));

// Create React Query client
const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      refetchOnWindowFocus: false,
      retry: 1,
      staleTime: 60000, // 60 seconds - longer stale time since WebSocket provides real-time updates
    },
  },
});

// Create Material-UI theme based on mode
const createAppTheme = (mode: ThemeMode) =>
  createTheme({
    palette: {
      mode,
      primary: {
        main: '#3f51b5',
      },
      secondary: {
        main: '#f50057',
      },
      background:
        mode === 'dark'
          ? {
              default: '#0a1929',
              paper: '#132f4c',
            }
          : {
              default: '#f5f5f5',
              paper: '#ffffff',
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
  // Initialize theme from localStorage or default to dark
  const [mode, setMode] = useState<ThemeMode>(() => {
    const stored = localStorage.getItem('theme');
    return (stored === 'light' || stored === 'dark') ? stored : 'dark';
  });

  const toggleTheme = () => {
    setMode((prevMode) => {
      const newMode = prevMode === 'dark' ? 'light' : 'dark';
      localStorage.setItem('theme', newMode);
      return newMode;
    });
  };

  const theme = useMemo(() => createAppTheme(mode), [mode]);

  const themeContextValue = useMemo(
    () => ({ mode, toggleTheme }),
    [mode]
  );

  return (
    <QueryClientProvider client={queryClient}>
      <ThemeContext.Provider value={themeContextValue}>
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
              path="/settings"
              element={
                <ProtectedRoute>
                  <UserSettings />
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
            <Route
              path="/admin/audit"
              element={
                <AdminRoute>
                  <AuditLogs />
                </AdminRoute>
              }
            />
            <Route
              path="/admin/settings"
              element={
                <AdminRoute>
                  <Settings />
                </AdminRoute>
              }
            />

            {/* Admin Content Management Routes */}
            <Route
              path="/admin/applications"
              element={
                <AdminRoute>
                  <Applications />
                </AdminRoute>
              }
            />
            <Route
              path="/admin/repositories"
              element={
                <AdminRoute>
                  <EnhancedRepositories />
                </AdminRoute>
              }
            />
            <Route
              path="/admin/plugin-catalog"
              element={
                <AdminRoute>
                  <PluginCatalog />
                </AdminRoute>
              }
            />
            <Route
              path="/admin/installed-plugins"
              element={
                <AdminRoute>
                  <InstalledPlugins />
                </AdminRoute>
              }
            />
            <Route
              path="/admin/scheduling"
              element={
                <AdminRoute>
                  <Scheduling />
                </AdminRoute>
              }
            />
            <Route
              path="/admin/security"
              element={
                <AdminRoute>
                  <SecuritySettings />
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
      </ThemeContext.Provider>
    </QueryClientProvider>
  );
}

export default App;
