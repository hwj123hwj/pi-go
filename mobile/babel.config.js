module.exports = function (api) {
  api.cache(true);
  return {
    presets: [
      ['babel-preset-expo', { jsxImportSource: false }],
    ],
    plugins: [
      // ── js-react-compiler (HIGH): Automatic memoization ──
      // Eliminates need for manual memo/useMemo/useCallback in most cases.
      // Requires React 18.3+ and React Native 0.76+ (both satisfied by Expo SDK 52).
      ['react-compiler', { target: '18' }],
    ],
  };
};
