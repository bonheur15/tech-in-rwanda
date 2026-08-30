export default {
  plugins: ["prettier-plugin-sh", "prettier-plugin-astro"],
  overrides: [
    {
      files: ["*.astro"],
      options: { parser: "astro" },
    },
    {
      files: ["Dockerfile", "*.dockerfile", ".dockerignore"],
      options: { parser: "dockerfile" },
    },
  ],
};
