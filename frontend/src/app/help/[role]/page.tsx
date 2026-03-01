import fs from 'fs';
import path from 'path';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import styles from '../layout.module.css';

// Using Server Component to read file directly
export default async function HelpRolePage({
  params,
}: {
  params: Promise<{ role: string }>
}) {
  const resolvedParams = await params;
  const role = resolvedParams.role;
  const validRoles = ['student', 'teacher', 'school_admin', 'sys_admin'];
  
  if (!validRoles.includes(role)) {
    return <div>找不到该角色的文档</div>;
  }

  const filename = `${role.toUpperCase()}_MANUAL.md`;
  // Read from the copied public directory to ensure it is available in production
  const filePath = path.join(process.cwd(), 'public', 'docs', filename);
  
  let content = '';
  try {
    content = fs.readFileSync(filePath, 'utf8');
  } catch (e) {
    content = '# 错误\n\n无法加载该文档，请联系管理员。';
    console.error(`Failed to read markdown file: ${filePath}`, e);
  }

  return (
    <div className={styles.markdownContainer}>
      <ReactMarkdown remarkPlugins={[remarkGfm]}>
        {content}
      </ReactMarkdown>
    </div>
  );
}
