import { defineConfig } from "vitepress";

export default defineConfig({
  title: "Painter-2026 Handbook",
  description: "Full architecture, development, testing, and deployment guide.",
  themeConfig: {
    nav: [
      { text: "概览", link: "/" },
      { text: "架构", link: "/architecture/overview" },
      { text: "开发", link: "/guides/development" },
      { text: "测试", link: "/guides/testing" },
      { text: "迁移", link: "/migration/legacy-api-matrix" },
      { text: "部署", link: "/deploy/overview" },
    ],
    sidebar: [
      {
        text: "开始",
        items: [
          { text: "项目概览", link: "/" },
          { text: "快速开始", link: "/guides/quick-start" },
        ],
      },
      {
        text: "架构",
        items: [
          { text: "架构总览", link: "/architecture/overview" },
          { text: "数据流", link: "/architecture/data-flow" },
          { text: "服务边界", link: "/architecture/service-boundary" },
        ],
      },
      {
        text: "开发与测试",
        items: [
          { text: "开发指南", link: "/guides/development" },
          { text: "测试指南", link: "/guides/testing" },
          { text: "契约与 SDK", link: "/guides/contract-sdk" },
        ],
      },
      {
        text: "部署",
        items: [
          { text: "部署总览", link: "/deploy/overview" },
          { text: "构建发布", link: "/deploy/build-release" },
          { text: "回滚手册", link: "/deploy/rollback" },
        ],
      },
      {
        text: "复刻迁移",
        items: [
          { text: "老新接口矩阵", link: "/migration/legacy-api-matrix" },
          { text: "复刻差异记录", link: "/migration/replication-diff" },
        ],
      },
    ],
  },
});
