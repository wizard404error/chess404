import React, {useRef, useState, useCallback, useEffect} from 'react';
import {
  SafeAreaView,
  StatusBar,
  StyleSheet,
  View,
  Text,
  ActivityIndicator,
  BackHandler,
  Platform,
  Linking,
  KeyboardAvoidingView,
} from 'react-native';
import {WebView} from 'react-native-webview';
import type {WebViewNavigation, WebViewMessageEvent} from 'react-native-webview';

// Set CHESS404_URL env var at build time for Railway/non-dev deployments.
const PRODUCTION_URL = process.env.CHESS404_URL || 'https://chess404.app';
const DEV_URL = 'http://10.0.2.2:3000';

const WEB_URL = __DEV__ ? DEV_URL : PRODUCTION_URL;

const CONNECTION_TIMEOUT_MS = 15000;

const INJECTED_JS = `
  (function() {
    window.ReactNativeWebView.postMessage(JSON.stringify({ type: 'ready' }));
  })();
`;

function App(): React.JSX.Element {
  const webViewRef = useRef<WebView>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [canGoBack, setCanGoBack] = useState(false);

  // Connection timeout: if no 'ready' message within 15s, show error
  useEffect(() => {
    if (!loading) return;
    const timer = setTimeout(() => {
      setError('Connection timed out. Check your network and that the server is running.');
      setLoading(false);
    }, CONNECTION_TIMEOUT_MS);
    return () => clearTimeout(timer);
  }, [loading]);

  // Deep link handling: open match URLs in the WebView
  useEffect(() => {
    const handleDeepLink = (event: {url: string}) => {
      const url = event.url;
      if (url.startsWith(WEB_URL) && webViewRef.current) {
        webViewRef.current.loadUrl(url);
      }
    };
    const subscription = Linking.addEventListener('url', handleDeepLink);
    Linking.getInitialURL().then((url) => {
      if (url && url.startsWith(WEB_URL) && webViewRef.current) {
        webViewRef.current.loadUrl(url);
      }
    });
    return () => subscription.remove();
  }, []);

  const onNavigationStateChange = useCallback((navState: WebViewNavigation) => {
    setCanGoBack(navState.canGoBack);
  }, []);

  const onMessage = useCallback((event: WebViewMessageEvent) => {
    try {
      const data = JSON.parse(event.nativeEvent.data);
      if (data.type === 'ready') {
        setLoading(false);
      }
    } catch {}
  }, []);

  const onShouldStartLoadWithRequest = useCallback((request: WebViewNavigation) => {
    const url = request.url;
    if (url.startsWith('http://') || url.startsWith('https://')) {
      return true;
    }
    return false;
  }, []);

  useEffect(() => {
    const backHandler = BackHandler.addEventListener('hardwareBackPress', () => {
      if (canGoBack && webViewRef.current) {
        webViewRef.current.goBack();
        return true;
      }
      return false;
    });
    return () => backHandler.remove();
  }, [canGoBack]);

  return (
    <SafeAreaView style={styles.container}>
      <StatusBar barStyle="light-content" backgroundColor="#0a0e1a" />
      {loading && (
        <View style={styles.loadingContainer}>
          <Text style={styles.logoText}>CHESS404</Text>
          <ActivityIndicator size="large" color="#ffbe5a" style={styles.spinner} />
          <Text style={styles.loadingText}>Loading...</Text>
        </View>
      )}
      {error && (
        <View style={styles.errorContainer}>
          <Text style={styles.errorIcon}>⚠</Text>
          <Text style={styles.errorTitle}>Connection Error</Text>
          <Text style={styles.errorText}>{error}</Text>
          <Text
            style={styles.retryText}
            onPress={() => {
              setError(null);
              setLoading(true);
              webViewRef.current?.reload();
            }}>
            Tap to retry
          </Text>
        </View>
      )}
      <KeyboardAvoidingView
        style={styles.flex}
        behavior={Platform.OS === 'ios' ? 'padding' : undefined}
        keyboardVerticalOffset={Platform.OS === 'ios' ? 0 : 0}>
        <WebView
          ref={webViewRef}
          source={{uri: WEB_URL}}
          style={[styles.webview, loading && styles.hidden]}
          injectedJavaScript={INJECTED_JS}
          onMessage={onMessage}
          onNavigationStateChange={onNavigationStateChange}
          onShouldStartLoadWithRequest={onShouldStartLoadWithRequest}
          onError={(syntheticEvent) => {
            const {nativeEvent} = syntheticEvent;
            setError(nativeEvent.description || 'Failed to connect to Chess404 server');
            setLoading(false);
          }}
          onLoadEnd={() => setLoading(false)}
          javaScriptEnabled
          domStorageEnabled
          startInLoadingState={false}
          allowsInlineMediaPlayback
          mediaPlaybackRequiresUserAction={false}
          allowsBackForwardNavigationGestures
          overScrollMode="never"
          mixedContentMode="never"
          allowFileAccess={false}
          allowUniversalAccessFromFileURLs={false}
          cacheEnabled
          incognito={false}
          applicationNameForUserAgent="Chess404Mobile/1.0"
          pullToRefreshEnabled
        />
      </KeyboardAvoidingView>
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: '#0a0e1a',
  },
  webview: {
    flex: 1,
  },
  hidden: {
    opacity: 0,
  },
  loadingContainer: {
    flex: 1,
    justifyContent: 'center',
    alignItems: 'center',
    backgroundColor: '#0a0e1a',
    position: 'absolute',
    top: 0,
    left: 0,
    right: 0,
    bottom: 0,
    zIndex: 10,
  },
  logoText: {
    fontSize: 36,
    fontWeight: '900',
    color: '#ffbe5a',
    letterSpacing: 4,
    marginBottom: 24,
  },
  spinner: {
    marginBottom: 16,
  },
  loadingText: {
    color: '#bbbbba',
    fontSize: 14,
  },
  errorContainer: {
    flex: 1,
    justifyContent: 'center',
    alignItems: 'center',
    backgroundColor: '#0a0e1a',
    position: 'absolute',
    top: 0,
    left: 0,
    right: 0,
    bottom: 0,
    zIndex: 10,
    padding: 32,
  },
  flex: {
    flex: 1,
  },
  errorIcon: {
    fontSize: 40,
    marginBottom: 12,
  },
  errorTitle: {
    fontSize: 20,
    fontWeight: '700',
    color: '#ff6b6b',
    marginBottom: 12,
  },
  errorText: {
    color: '#bbbbba',
    fontSize: 14,
    textAlign: 'center',
    marginBottom: 24,
  },
  retryText: {
    color: '#ffbe5a',
    fontSize: 16,
    fontWeight: '600',
  },
});

export default App;
