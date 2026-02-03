import { Link } from 'react-router-dom';
import { useAuthStore } from '@/stores/authStore';
import { ArrowRight, Code2, Timer, Trophy, Sparkles } from 'lucide-react';

export default function Home() {
    const isAuthenticated = useAuthStore((state) => state.isAuthenticated);

    const features = [
        {
            icon: Code2,
            title: 'NeetCode 150 Problems',
            description: 'Curated problems from the famous NeetCode 150 list, covering all essential DSA patterns.',
        },
        {
            icon: Timer,
            title: 'Timed Contests',
            description: 'Challenge yourself with customizable time limits and problem counts.',
        },
        {
            icon: Trophy,
            title: 'Progress Tracking',
            description: 'Never repeat solved problems. Track your progress across all topics.',
        },
        {
            icon: Sparkles,
            title: 'Smart Selection',
            description: 'Problems selected with gradual difficulty increase for optimal learning.',
        },
    ];

    return (
        <div className="min-h-screen bg-[var(--color-background)]">
            {/* Hero Section */}
            <div className="relative overflow-hidden">
                {/* Background gradient */}
                <div className="absolute inset-0 bg-gradient-to-br from-[var(--color-primary)]/20 via-transparent to-[var(--color-secondary)]/20" />

                {/* Animated circles */}
                <div className="absolute top-20 left-10 w-72 h-72 bg-[var(--color-primary)]/30 rounded-full blur-3xl animate-pulse" />
                <div className="absolute bottom-20 right-10 w-96 h-96 bg-[var(--color-secondary)]/20 rounded-full blur-3xl animate-pulse" style={{ animationDelay: '1s' }} />

                <div className="relative max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-24 sm:py-32">
                    <div className="text-center">
                        {/* Logo */}
                        <div className="inline-flex items-center justify-center w-20 h-20 rounded-2xl bg-gradient-to-br from-[var(--color-primary)] to-[var(--color-secondary)] mb-8 animate-pulse-glow">
                            <span className="text-white font-bold text-2xl">150</span>
                        </div>

                        {/* Heading */}
                        <h1 className="text-4xl sm:text-6xl font-bold tracking-tight mb-6">
                            Master DSA with
                            <br />
                            <span className="gradient-text">Timed Contests</span>
                        </h1>

                        {/* Subheading */}
                        <p className="text-lg sm:text-xl text-[var(--color-text-muted)] max-w-2xl mx-auto mb-10">
                            Practice problems from NeetCode 150 in a contest format.
                            Smart problem selection ensures you cover all topics without repetition.
                        </p>

                        {/* CTA Buttons */}
                        <div className="flex flex-col sm:flex-row items-center justify-center gap-4">
                            {isAuthenticated ? (
                                <Link to="/dashboard" className="btn btn-primary text-lg px-8 py-4">
                                    Go to Dashboard
                                    <ArrowRight className="w-5 h-5" />
                                </Link>
                            ) : (
                                <>
                                    <Link to="/signup" className="btn btn-primary text-lg px-8 py-4">
                                        Get Started Free
                                        <ArrowRight className="w-5 h-5" />
                                    </Link>
                                    <Link to="/login" className="btn btn-secondary text-lg px-8 py-4">
                                        Sign In
                                    </Link>
                                </>
                            )}
                        </div>
                    </div>
                </div>
            </div>

            {/* Features Section */}
            <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-24">
                <div className="text-center mb-16">
                    <h2 className="text-3xl sm:text-4xl font-bold mb-4">
                        Everything you need to
                        <span className="gradient-text"> ace interviews</span>
                    </h2>
                    <p className="text-[var(--color-text-muted)] max-w-2xl mx-auto">
                        A structured approach to mastering coding problems, designed for efficient interview preparation.
                    </p>
                </div>

                <div className="grid md:grid-cols-2 lg:grid-cols-4 gap-6">
                    {features.map((feature, index) => {
                        const Icon = feature.icon;
                        return (
                            <div
                                key={feature.title}
                                className="card group animate-fadeIn"
                                style={{ animationDelay: `${index * 0.1}s` }}
                            >
                                <div className="w-12 h-12 rounded-xl bg-gradient-to-br from-[var(--color-primary)] to-[var(--color-secondary)] flex items-center justify-center mb-4 group-hover:scale-110 transition-transform">
                                    <Icon className="w-6 h-6 text-white" />
                                </div>
                                <h3 className="text-lg font-semibold mb-2">{feature.title}</h3>
                                <p className="text-sm text-[var(--color-text-muted)]">
                                    {feature.description}
                                </p>
                            </div>
                        );
                    })}
                </div>
            </div>

            {/* Stats Section */}
            <div className="border-y border-[var(--color-border)] bg-[var(--color-surface)]/50">
                <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-16">
                    <div className="grid grid-cols-2 md:grid-cols-4 gap-8 text-center">
                        <div>
                            <div className="text-4xl font-bold gradient-text mb-2">150</div>
                            <div className="text-sm text-[var(--color-text-muted)]">Curated Problems</div>
                        </div>
                        <div>
                            <div className="text-4xl font-bold gradient-text mb-2">17</div>
                            <div className="text-sm text-[var(--color-text-muted)]">Topics Covered</div>
                        </div>
                        <div>
                            <div className="text-4xl font-bold gradient-text mb-2">∞</div>
                            <div className="text-sm text-[var(--color-text-muted)]">Contests</div>
                        </div>
                        <div>
                            <div className="text-4xl font-bold gradient-text mb-2">Free</div>
                            <div className="text-sm text-[var(--color-text-muted)]">Forever</div>
                        </div>
                    </div>
                </div>
            </div>

            {/* CTA Section */}
            <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-24">
                <div className="gradient-border p-8 sm:p-12 text-center">
                    <h2 className="text-2xl sm:text-3xl font-bold mb-4">
                        Ready to start practicing?
                    </h2>
                    <p className="text-[var(--color-text-muted)] mb-8 max-w-xl mx-auto">
                        Create your first contest in seconds and start your journey to mastering DSA.
                    </p>
                    {isAuthenticated ? (
                        <Link to="/contest/create" className="btn btn-primary">
                            Create Your First Contest
                            <ArrowRight className="w-4 h-4" />
                        </Link>
                    ) : (
                        <Link to="/signup" className="btn btn-primary">
                            Sign Up Now
                            <ArrowRight className="w-4 h-4" />
                        </Link>
                    )}
                </div>
            </div>

            {/* Footer */}
            <footer className="border-t border-[var(--color-border)]">
                <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
                    <div className="flex flex-col sm:flex-row items-center justify-between gap-4">
                        <div className="flex items-center gap-2">
                            <div className="w-8 h-8 rounded-lg bg-gradient-to-br from-[var(--color-primary)] to-[var(--color-secondary)] flex items-center justify-center">
                                <span className="text-white font-bold text-xs">150</span>
                            </div>
                            <span className="font-semibold">Contest Maker</span>
                        </div>
                        <p className="text-sm text-[var(--color-text-muted)]">
                            Built for developers, by developers. Open source.
                        </p>
                    </div>
                </div>
            </footer>
        </div>
    );
}
