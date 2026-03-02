'use client';

import DashboardLayout from '@/components/DashboardLayout';

export default function AdminLayout({ children }: { children: React.ReactNode }) {
    return <DashboardLayout variant="admin">{children}</DashboardLayout>;
}
