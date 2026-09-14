import { Component } from 'react';
import type { ReactNode } from 'react';
import { AlertCircle, RefreshCw } from 'lucide-react';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';

interface Props {
  children: ReactNode;
  fallback?: ReactNode;
}

interface State {
  hasError: boolean;
  error?: Error;
  errorInfo?: React.ErrorInfo;
}

export class ErrorBoundary extends Component<Props, State> {
  constructor(props: Props) {
    super(props);
    this.state = { hasError: false };
  }

  static getDerivedStateFromError(error: Error): Partial<State> {
    return { hasError: true, error };
  }

  // Stale-module errors — a lazy chunk whose hash changed after a redeploy or a
  // dev dep re-optimization, leaving the tab with a dangling module graph — are
  // only fixable by a full reload; "Try Again" just re-renders the same broken
  // graph. Detect that class and reload once (guarded against a reload loop so a
  // genuine, repeating render bug still surfaces the manual fallback).
  private static isStaleModuleError(error: unknown): boolean {
    const message = error instanceof Error ? error.message : String(error ?? '');
    return /failed to fetch dynamically imported module|error loading dynamically imported module|importing a module script failed|reading 'useContext'/i.test(message);
  }

  private static tryReloadOnce(): boolean {
    const key = 'app-stale-module-reload-at';
    const last = Number(window.sessionStorage.getItem(key) || 0);
    const now = Date.now();
    if (now - last > 10000) {
      window.sessionStorage.setItem(key, String(now));
      window.location.reload();
      return true;
    }
    return false;
  }

  componentDidCatch(error: Error, errorInfo: React.ErrorInfo): void {
    console.error('ErrorBoundary caught an error:', error, errorInfo);
    if (ErrorBoundary.isStaleModuleError(error) && ErrorBoundary.tryReloadOnce()) {
      return;
    }
    this.setState({ errorInfo });
  }

  handleRetry = (): void => {
    this.setState({ hasError: false, error: undefined, errorInfo: undefined });
  };

  handleReload = (): void => {
    window.location.reload();
  };

  render(): ReactNode {
    if (this.state.hasError) {
      if (this.props.fallback) {
        return this.props.fallback;
      }

      return (
        <div className="flex items-center justify-center min-h-[400px] p-4">
          <div className="w-full max-w-lg">
            <Alert variant="destructive">
              <AlertCircle className="h-4 w-4" />
              <AlertTitle>Something went wrong</AlertTitle>
              <AlertDescription className="mt-2">
                <p className="text-sm mb-4">
                  {this.state.error?.message || 'An unexpected error occurred'}
                </p>
                {process.env.NODE_ENV === 'development' && this.state.errorInfo && (
                  <details className="mt-2">
                    <summary className="cursor-pointer text-xs opacity-70 hover:opacity-100">
                      Stack trace
                    </summary>
                    <pre className="mt-2 text-xs overflow-auto max-h-40 p-2 bg-black/10 rounded">
                      {this.state.errorInfo.componentStack}
                    </pre>
                  </details>
                )}
              </AlertDescription>
            </Alert>
            <div className="flex gap-3 mt-4 justify-center">
              <Button onClick={this.handleRetry} variant="outline">
                <RefreshCw className="h-4 w-4 mr-2" />
                Try Again
              </Button>
              <Button onClick={this.handleReload} variant="default">
                Reload Page
              </Button>
            </div>
          </div>
        </div>
      );
    }

    return this.props.children;
  }
}

// Functional wrapper for easier use with hooks
export function withErrorBoundary<P extends object>(
  WrappedComponent: React.ComponentType<P>,
  fallback?: ReactNode
) {
  return function WithErrorBoundary(props: P) {
    return (
      <ErrorBoundary fallback={fallback}>
        <WrappedComponent {...props} />
      </ErrorBoundary>
    );
  };
}
