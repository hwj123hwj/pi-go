const { getDefaultConfig } = require('expo/metro-config');

const config = getDefaultConfig(__dirname);

// ── RN Best Practice: Tree Shaking (bundle-tree-shaking) ──────────────
// Enable experimental import/export elimination for production builds.
// Reduces bundle size by removing unused exported code from dependencies.
config.transformer.getTransformOptions = async () => ({
  transform: {
    experimentalImportSupport: true,
    inlineRequires: true,
  },
});

module.exports = config;
