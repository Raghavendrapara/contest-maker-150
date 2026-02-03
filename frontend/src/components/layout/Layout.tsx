import { Link, useLocation, useNavigate } from 'react-router-dom';
import { useAuthStore } from '@/stores/authStore';
import {
    Home,
    Trophy,
    PlusCircle,
    History,
    LogOut,
    User,
    Menu,
    X
} from 'lucide-react';
import { useState } from 'react';
import clsx from 'clsx';

interface LayoutProps {
    children: React.ReactNode;
}

export default function Layout({ children }: LayoutProps) {
    const location = useLocation();
    const navigate = useNavigate();
    const { user, logout } = useAuthStore();
    const [isMobileMenuOpen, setIsMobileMenuOpen] = useState(false);

    const handleLogout = () => {
        logout();
        navigate('/');
    };

    const navItems = [
        { path: '/dashboard', label: 'Dashboard', icon: Home },
        { path: '/contest/create', label: 'New Contest', icon: PlusCircle },
        { path: '/contest/active', label: 'Active Contest', icon: Trophy },
        { path: '/history', label: 'History', icon: History },
    ];

    return (
        <div className="min-h-screen bg-[var(--color-background)]">
            {/* Header */}
            <header className="glass sticky top-0 z-50">
                <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
                    <div className="flex items-center justify-between h-16">
                        {/* Logo */}
                        <Link to="/dashboard" className="flex items-center gap-3">
                            <div className="w-10 h-10 rounded-xl bg-gradient-to-br from-[var(--color-primary)] to-[var(--color-secondary)] flex items-center justify-center">
                                <span className="text-white font-bold text-sm">150</span>
                            </div>
                            <span className="font-bold text-xl hidden sm:block">
                                Contest Maker
                            </span>
                        </Link>

                        {/* Desktop Navigation */}
                        <nav className="hidden md:flex items-center gap-1">
                            {navItems.map((item) => {
                                const Icon = item.icon;
                                const isActive = location.pathname === item.path;
                                return (
                                    <Link
                                        key={item.path}
                                        to={item.path}
                                        className={clsx(
                                            'flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-medium transition-all',
                                            isActive
                                                ? 'bg-[var(--color-primary)] text-white'
                                                : 'text-[var(--color-text-muted)] hover:text-white hover:bg-[var(--color-surface-hover)]'
                                        )}
                                    >
                                        <Icon className="w-4 h-4" />
                                        {item.label}
                                    </Link>
                                );
                            })}
                        </nav>

                        {/* User Menu */}
                        <div className="flex items-center gap-4">
                            <div className="hidden sm:flex items-center gap-2 text-sm text-[var(--color-text-muted)]">
                                <User className="w-4 h-4" />
                                <span>{user?.username}</span>
                            </div>
                            <button
                                onClick={handleLogout}
                                className="btn-ghost p-2 rounded-lg"
                                title="Logout"
                            >
                                <LogOut className="w-5 h-5" />
                            </button>

                            {/* Mobile menu button */}
                            <button
                                onClick={() => setIsMobileMenuOpen(!isMobileMenuOpen)}
                                className="md:hidden p-2 rounded-lg hover:bg-[var(--color-surface-hover)]"
                            >
                                {isMobileMenuOpen ? (
                                    <X className="w-6 h-6" />
                                ) : (
                                    <Menu className="w-6 h-6" />
                                )}
                            </button>
                        </div>
                    </div>
                </div>

                {/* Mobile Navigation */}
                {isMobileMenuOpen && (
                    <div className="md:hidden border-t border-[var(--color-border)] animate-fadeIn">
                        <nav className="px-4 py-4 space-y-2">
                            {navItems.map((item) => {
                                const Icon = item.icon;
                                const isActive = location.pathname === item.path;
                                return (
                                    <Link
                                        key={item.path}
                                        to={item.path}
                                        onClick={() => setIsMobileMenuOpen(false)}
                                        className={clsx(
                                            'flex items-center gap-3 px-4 py-3 rounded-lg text-sm font-medium transition-all',
                                            isActive
                                                ? 'bg-[var(--color-primary)] text-white'
                                                : 'text-[var(--color-text-muted)] hover:text-white hover:bg-[var(--color-surface-hover)]'
                                        )}
                                    >
                                        <Icon className="w-5 h-5" />
                                        {item.label}
                                    </Link>
                                );
                            })}
                        </nav>
                    </div>
                )}
            </header>

            {/* Main Content */}
            <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
                {children}
            </main>

            {/* Footer */}
            <footer className="border-t border-[var(--color-border)] mt-auto">
                <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-6">
                    <p className="text-center text-sm text-[var(--color-text-muted)]">
                        Contest Maker 150 — Practice NeetCode 150 problems with timed contests
                    </p>
                </div>
            </footer>
        </div>
    );
}
