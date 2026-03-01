import { redirect } from 'next/navigation';

export default function HelpIndexPage() {
  // 默认重定向到通用说明页或某一个角色的页面
  redirect('/help/student');
}
