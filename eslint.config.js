import security from "eslint-plugin-security";

export default [
  security.configs.recommended,
  {
    files: ["cmd/tapedeck/web/*.js"],
    languageOptions: {
      ecmaVersion: "latest",
      sourceType: "script",
      globals: {
        window: "readonly",
        document: "readonly",
        navigator: "readonly",
        localStorage: "readonly",
        indexedDB: "readonly",
        console: "readonly",
        fetch: "readonly",
        Audio: "readonly",
        Blob: "readonly",
        URL: "readonly",
        URLSearchParams: "readonly",
        Request: "readonly",
        Response: "readonly",
        caches: "readonly",
        self: "readonly",
        clients: "readonly",
        Promise: "readonly",
        setTimeout: "readonly",
        setInterval: "readonly",
        clearInterval: "readonly",
        alert: "readonly",
        IDBKeyRange: "readonly",
        history: "readonly",
        location: "readonly",
      },
    },
    rules: {
      "no-unused-vars": ["error", { argsIgnorePattern: "^_", caughtErrorsIgnorePattern: "^_" }],
      "no-undef": "error",
      "no-redeclare": "error",
      eqeqeq: ["warn", "smart"],
    },
  },
];
