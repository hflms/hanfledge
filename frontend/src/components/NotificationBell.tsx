'use client';

import { useEffect, useState } from 'react';
import { apiFetch } from '@/lib/api';
import styles from './NotificationBell.module.css';

interface Notification {
  id: number;
  type: string;
  title: string;
  content: string;
  is_read: boolean;
  created_at: string;
}

export default function NotificationBell() {
  const [notifications, setNotifications] = useState<Notification[]>([]);
  const [showDropdown, setShowDropdown] = useState(false);

  useEffect(() => {
    let mounted = true;
    const loadNotifications = async () => {
      try {
        const data = await apiFetch<{ notifications: Notification[] }>('/notifications/unread');
        if (mounted) {
          setNotifications(data.notifications || []);
        }
      } catch (err) {
        console.error('Failed to load notifications:', err);
      }
    };
    loadNotifications();
    const interval = setInterval(loadNotifications, 60000); // 每分钟刷新
    return () => {
      mounted = false;
      clearInterval(interval);
    };
  }, []);

  const markAsRead = async (id: number) => {
    try {
      await apiFetch(`/notifications/${id}/read`, { method: 'POST' });
      setNotifications(prev => prev.filter(n => n.id !== id));
    } catch (err) {
      console.error('Failed to mark as read:', err);
    }
  };

  const unreadCount = notifications.length;

  return (
    <div className={styles.container}>
      <button 
        className={styles.bell}
        onClick={() => setShowDropdown(!showDropdown)}
        aria-label={unreadCount > 0 ? `通知（${unreadCount}条未读）` : '通知'}
        aria-expanded={showDropdown}
        aria-haspopup="true"
      >
        <span aria-hidden="true">🔔</span>
        {unreadCount > 0 && <span className={styles.badge} aria-hidden="true">{unreadCount}</span>}
      </button>

      {showDropdown && (
        <div className={styles.dropdown}>
          <div className={styles.header}>通知</div>
          {notifications.length === 0 ? (
            <div className={styles.empty}>暂无通知</div>
          ) : (
            notifications.map(n => (
              <div key={n.id} className={styles.item}>
                <div className={styles.title}>{n.title}</div>
                <div className={styles.content}>{n.content.slice(0, 100)}...</div>
                <button 
                  className={styles.markRead}
                  onClick={() => markAsRead(n.id)}
                  aria-label={`将“${n.title}”标记为已读`}
                >
                  标记已读
                </button>
              </div>
            ))
          )}
        </div>
      )}
    </div>
  );
}
