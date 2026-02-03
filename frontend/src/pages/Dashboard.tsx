import { useQuery } from '@tanstack/react-query';
import { Link } from 'react-router-dom';
import { userApi, contestApi } from '@/services/api';
import { useAuthStore } from '@/stores/authStore';
import {
    Trophy,
    Target,
    CheckCircle2,
    Clock,
    ArrowRight,
    TrendingUp,
    Zap
} from 'lucide-react';
import clsx from 'clsx';
import type { UserProgress, Contest } from '@/types';

export default function Dashboard() {
    const user = useAuthStore((state) => state.user);

    const { data: progress, isLoading: progressLoading } = useQuery<UserProgress>({
        queryKey: ['user-progress'],
        queryFn: userApi.getProgress,
    });

    const { data: activeContestData, isLoading: contestLoading } = useQuery<{ contest: Contest | null }>({
        queryKey: ['active-contest'],
        queryFn: contestApi.getActive,
    });

    const activeContest = activeContestData?.contest;

    // Calculate solved percentages
    const totalProblems = 150;
    const solvedPercentage = progress ? (progress.total_solved / totalProblems) * 100 : 0;

    const difficultyStats = [
        {
            label: 'Easy',
            solved: progress?.easy_solved ?? 0,
            total: 45,
            color: 'var(--color-easy)',
            bgColor: 'rgba(34, 197, 94, 0.15)'
        },
        {
            label: 'Medium',
            solved: progress?.medium_solved ?? 0,
            total: 80,
            color: 'var(--color-medium)',
            bgColor: 'rgba(245, 158, 11, 0.15)'
        },
        {
            label: 'Hard',
            solved: progress?.hard_solved ?? 0,
            total: 25,
            color: 'var(--color-hard)',
            bgColor: 'rgba(239, 68, 68, 0.15)'
        },
    ];

    return (
        <div className="space-y-8 animate-fadeIn">
            {/* Welcome Section */}
            <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
                <div>
                    <h1 className="text-3xl font-bold">
                        Welcome back, <span className="gradient-text">{user?.username}</span>!
                    </h1>
                    <p className="text-[var(--color-text-muted)] mt-1">
                        Ready for your next challenge?
                    </p>
                </div>
                <Link to="/contest/create" className="btn btn-primary">
                    <Zap className="w-4 h-4" />
                    Start New Contest
                </Link>
            </div>

            {/* Active Contest Banner */}
            {activeContest && (
                <div className="gradient-border p-6 rounded-xl bg-gradient-to-r from-[var(--color-primary)]/10 to-[var(--color-secondary)]/10">
                    <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
                        <div className="flex items-center gap-4">
                            <div className="w-12 h-12 rounded-full bg-[var(--color-primary)] flex items-center justify-center animate-pulse">
                                <Clock className="w-6 h-6 text-white" />
                            </div>
                            <div>
                                <h3 className="font-semibold text-lg">Active Contest</h3>
                                <p className="text-[var(--color-text-muted)]">
                                    {activeContest.problems.filter(p => p.is_completed).length} of {activeContest.problems.length} problems completed
                                </p>
                            </div>
                        </div>
                        <Link to="/contest/active" className="btn btn-primary">
                            Continue
                            <ArrowRight className="w-4 h-4" />
                        </Link>
                    </div>
                </div>
            )}

            {/* Stats Grid */}
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
                {/* Total Progress */}
                <div className="card">
                    <div className="flex items-center justify-between mb-4">
                        <span className="text-[var(--color-text-muted)] text-sm font-medium">
                            Total Progress
                        </span>
                        <TrendingUp className="w-5 h-5 text-[var(--color-primary)]" />
                    </div>
                    <div className="text-3xl font-bold mb-2">
                        {progressLoading ? (
                            <div className="h-9 w-20 skeleton rounded" />
                        ) : (
                            <>
                                {progress?.total_solved ?? 0}
                                <span className="text-lg text-[var(--color-text-muted)]"> / {totalProblems}</span>
                            </>
                        )}
                    </div>
                    <div className="h-2 bg-[var(--color-surface-hover)] rounded-full overflow-hidden">
                        <div
                            className="h-full bg-gradient-to-r from-[var(--color-primary)] to-[var(--color-secondary)] rounded-full transition-all duration-500"
                            style={{ width: `${solvedPercentage}%` }}
                        />
                    </div>
                </div>

                {/* Contests Completed */}
                <div className="card">
                    <div className="flex items-center justify-between mb-4">
                        <span className="text-[var(--color-text-muted)] text-sm font-medium">
                            Contests Completed
                        </span>
                        <Trophy className="w-5 h-5 text-[var(--color-warning)]" />
                    </div>
                    <div className="text-3xl font-bold">
                        {progressLoading ? (
                            <div className="h-9 w-12 skeleton rounded" />
                        ) : (
                            progress?.contest_stats?.completed_contests ?? 0
                        )}
                    </div>
                </div>

                {/* Total Contests */}
                <div className="card">
                    <div className="flex items-center justify-between mb-4">
                        <span className="text-[var(--color-text-muted)] text-sm font-medium">
                            Total Contests
                        </span>
                        <Target className="w-5 h-5 text-[var(--color-secondary)]" />
                    </div>
                    <div className="text-3xl font-bold">
                        {progressLoading ? (
                            <div className="h-9 w-12 skeleton rounded" />
                        ) : (
                            progress?.contest_stats?.total_contests ?? 0
                        )}
                    </div>
                </div>

                {/* Problems Left */}
                <div className="card">
                    <div className="flex items-center justify-between mb-4">
                        <span className="text-[var(--color-text-muted)] text-sm font-medium">
                            Problems Left
                        </span>
                        <CheckCircle2 className="w-5 h-5 text-[var(--color-success)]" />
                    </div>
                    <div className="text-3xl font-bold">
                        {progressLoading ? (
                            <div className="h-9 w-12 skeleton rounded" />
                        ) : (
                            totalProblems - (progress?.total_solved ?? 0)
                        )}
                    </div>
                </div>
            </div>

            {/* Difficulty Breakdown */}
            <div className="card">
                <h2 className="text-lg font-semibold mb-6">Difficulty Breakdown</h2>
                <div className="space-y-6">
                    {difficultyStats.map((stat) => {
                        const percentage = (stat.solved / stat.total) * 100;
                        return (
                            <div key={stat.label}>
                                <div className="flex items-center justify-between mb-2">
                                    <div className="flex items-center gap-2">
                                        <span
                                            className="w-3 h-3 rounded-full"
                                            style={{ backgroundColor: stat.color }}
                                        />
                                        <span className="font-medium">{stat.label}</span>
                                    </div>
                                    <span className="text-sm text-[var(--color-text-muted)]">
                                        {stat.solved} / {stat.total}
                                    </span>
                                </div>
                                <div className="h-3 rounded-full overflow-hidden" style={{ backgroundColor: stat.bgColor }}>
                                    <div
                                        className="h-full rounded-full transition-all duration-500"
                                        style={{
                                            width: `${percentage}%`,
                                            backgroundColor: stat.color
                                        }}
                                    />
                                </div>
                            </div>
                        );
                    })}
                </div>
            </div>

            {/* Quick Actions */}
            <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                <Link
                    to="/contest/create"
                    className="card group flex items-center gap-4 hover:border-[var(--color-primary)]"
                >
                    <div className="w-12 h-12 rounded-xl bg-gradient-to-br from-[var(--color-primary)] to-[var(--color-secondary)] flex items-center justify-center group-hover:scale-110 transition-transform">
                        <Zap className="w-6 h-6 text-white" />
                    </div>
                    <div className="flex-1">
                        <h3 className="font-semibold">Create New Contest</h3>
                        <p className="text-sm text-[var(--color-text-muted)]">
                            Practice with a fresh set of problems
                        </p>
                    </div>
                    <ArrowRight className="w-5 h-5 text-[var(--color-text-muted)] group-hover:text-white transition-colors" />
                </Link>

                <Link
                    to="/history"
                    className="card group flex items-center gap-4 hover:border-[var(--color-primary)]"
                >
                    <div className="w-12 h-12 rounded-xl bg-[var(--color-surface-hover)] flex items-center justify-center group-hover:scale-110 transition-transform">
                        <Trophy className="w-6 h-6 text-[var(--color-warning)]" />
                    </div>
                    <div className="flex-1">
                        <h3 className="font-semibold">View History</h3>
                        <p className="text-sm text-[var(--color-text-muted)]">
                            Review your past contests
                        </p>
                    </div>
                    <ArrowRight className="w-5 h-5 text-[var(--color-text-muted)] group-hover:text-white transition-colors" />
                </Link>
            </div>
        </div>
    );
}
