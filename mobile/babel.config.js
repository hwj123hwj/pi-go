module.exports = function (api) {
  api.cache(true);
  return {
    presets: [
      ['babel-preset-expo', { jsxImportSource: false }],
    ],
    // ── js-react-compiler REMOVED: react-compiler-runtime@19 beta
    // requires React 19, but project uses React 18.3.1 → crash on launch.
    // Manual React.memo + useCallback from R1 optimizations remain in place.
    plugins: [
    ],
  };
};
