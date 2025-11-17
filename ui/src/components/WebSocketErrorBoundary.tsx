/**
 * WebSocketErrorBoundary Component
 *
 * React error boundary specifically for WebSocket-related errors.
 * Provides graceful degradation and fallback UI when WebSocket fails.
 *
 * @component
 */
import { Component, ReactNode } from 'react';
import {
  Box,
  Alert,
  AlertTitle,
  Button,
  Typography,
  Paper,
} from '@mui/material';
import {
  Refresh as RefreshIcon,
  ErrorOutline as ErrorIcon,
} from '@mui/icons-material';

interface Props {
  children: ReactNode;
  fallback?: ReactNode;
  onError?: (error: Error, errorInfo: React.ErrorInfo) => void;
  showErrorDetails?: boolean;
}

interface State {
  hasError: boolean;
  error: Error | null;
  errorInfo: React.ErrorInfo | null;
  dismissed: boolean; // Track if user has dismissed the error
}

export default class WebSocketErrorBoundary extends Component<Props, State> {
  constructor(props: Props) {
    super(props);
    this.state = {
      hasError: false,
      error: null,
      errorInfo: null,
      dismissed: false,
    };
  }

  static getDerivedStateFromError(error: Error): Partial<State> {
    return {
      hasError: true,
      error,
    };
  }

  componentDidCatch(error: Error, errorInfo: React.ErrorInfo) {
    console.error('WebSocket Error Boundary caught an error:', error, errorInfo);

    this.setState({
      error,
      errorInfo,
    });

    // Call optional error callback
    if (this.props.onError) {
      this.props.onError(error, errorInfo);
    }
  }

  handleReset = () => {
    this.setState({
      hasError: false,
      error: null,
      errorInfo: null,
      dismissed: true, // Mark as dismissed
    });
  };

  render() {
    // If error was already dismissed, just render children without showing error UI
    if (this.state.hasError && this.state.dismissed) {
      console.warn('WebSocket error (dismissed):', this.state.error?.message);
      return this.props.children;
    }

    if (this.state.hasError) {
      // Use custom fallback if provided
      if (this.props.fallback) {
        return this.props.fallback;
      }

      // Default error UI
      return (
        <Box
          sx={{
            display: 'flex',
            justifyContent: 'center',
            alignItems: 'center',
            minHeight: 200,
            p: 3,
          }}
        >
          <Paper sx={{ p: 3, maxWidth: 600 }}>
            <Alert severity="error" icon={<ErrorIcon fontSize="large" />}>
              <AlertTitle>WebSocket Connection Error</AlertTitle>
              <Typography variant="body2" paragraph>
                There was an error with the real-time connection. The page will continue to work,
                but live updates may be unavailable.
              </Typography>

              {this.props.showErrorDetails && this.state.error && (
                <Box sx={{ mt: 2, p: 2, bgcolor: 'grey.100', borderRadius: 1 }}>
                  <Typography variant="caption" component="pre" sx={{ whiteSpace: 'pre-wrap' }}>
                    {this.state.error.toString()}
                  </Typography>
                </Box>
              )}

              <Box sx={{ mt: 2, display: 'flex', gap: 1 }}>
                <Button
                  variant="contained"
                  onClick={this.handleReset}
                  size="small"
                >
                  Continue Without Live Updates
                </Button>
                <Button
                  variant="outlined"
                  startIcon={<RefreshIcon />}
                  onClick={() => window.location.reload()}
                  size="small"
                >
                  Reload Page
                </Button>
              </Box>
            </Alert>
          </Paper>
        </Box>
      );
    }

    return this.props.children;
  }
}
