import type { Metadata } from "next";
import "./globals.css";
import Providers from "./Providers";

export const metadata: Metadata = {
  title: "Hanfledge - AI 智适应学习平台",
  description: "基于知识图谱和多智能体编排的 AI 原生教育平台，为每位学生提供个性化的苏格拉底式学习体验。",
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="zh-CN">
      <body>
        <Providers>{children}</Providers>
      </body>
    </html>
  );
}
