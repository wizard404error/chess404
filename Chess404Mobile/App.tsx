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
} from 'react-native';
import {WebView} from 'react-native-webview';
import type {WebViewNavigation, WebViewMessageEvent} from 'react-native-webview';

const PRODUCTION_URL = 'https://chess404.app';
const DEV_URL = 'http://10.0.2.2:3000';

const WEB_URL = __DEV__ ? DEV_URL : PRODUCTION_URL;

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
    Linking.openURL(url);
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
          <Text style={styles.errorIcon}>Connection Error</Text>
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
      />
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
    color: '#bbbbba0',
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
  errorIcon: {
    fontSize: 20,
    fontWeight: '700',
    color: '#ff6b6b',
    marginBottom: 12,
  },
  errorText: {
    color: '#bbbbba0',
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
