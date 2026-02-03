import { useQuery } from '@tanstack/react-query';
import { Link } from 'react-router-dom';
import { contestApi } from '@/services/api';
import type { Contest } from '@/types';
import {
    Trophy,
    XCircle,
    Clock,
    CheckCircle2,
    ChevronRight,
    Loader2,
    FolderOpen
} from 'lucide-react';
import { formatDistanceToNow } from 'date-fns';
import clsx from 'clsx';

export default function ContestHistory() {
    const { data, isLoading } = useQuery<{ contests: Contest[] }>({
        queryKey: ['contests'],
        queryFn: contestApi.getAll,
    });

    const contests = data?.contests ?? [];
    const pastContests = contests.filter(c => c.status !== 'active');

    if (isLoading) {
        return (
            <div className="flex items-center justify-center min-h-[60vh]">
                <Loader2 className="w-8 h-8 animate-spin text-[var(--color-primary)]" />
            </div>
        );
    }

    return (
        <div className="animate-fadeIn">
            {/* Header */}
            <div className="mb-8">
                <h1 className="text-3xl font-bold mb-2">Contest History</h1>
                <p className="text-[var(--color-text-muted)]">
                    Review your past contests and track your progress
                </p>
            </div>

            {/* Stats Summary */}
            <div className="grid grid-cols-1 md:grid-cols-3 gap-6 mb-8">
                <div className="card">
                    <div className="flex items-center gap-4">
                        <div className="w-12 h-12 rounded-xl bg-gradient-to-br from-[var(--color-primary)] to-[var(--color-secondary)] flex items-center justify-center">
                            <Trophy className="w-6 h-6 text-white" />
                        </div>
                        <div>
                            <div className="text-2xl font-bold">
                                {pastContests.filter(c => c.status === 'completed').length}
                            </div>
                            <div className="text-sm text-[var(--color-text-muted)]">
                                Completed
                            </div>
                        </div>
                    </div>
                </div>

                <div className="card">
                    <div className="flex items-center gap-4">
                        <div className="w-12 h-12 rounded-xl bg-[var(--color-surface-hover)] flex items-center justify-center">
                            <XCircle className="w-6 h-6 text-[var(--color-error)]" />
                        </div>
                        <div>
                            <div className="text-2xl font-bold">
                                {pastContests.filter(c => c.status === 'abandoned').length}
                            </div>
                            <div className="text-sm text-[var(--color-text-muted)]">
                                Abandoned
                            </div>
                        </div>
                    </div>
                </div>

                <div className="card">
                    <div className="flex items-center gap-4">
                        <div className="w-12 h-12 rounded-xl bg-[var(--color-surface-hover)] flex items-center justify-center">
                            <CheckCircle2 className="w-6 h-6 text-[var(--color-success)]" />
                        </div>
                        <div>
                            <div className="text-2xl font-bold">
                                {pastContests.reduce((acc, c) =>
                                    acc + c.problems.filter(p => p.is_completed).length, 0
                                )}
                            </div>
                            <div className="text-sm text-[var(--color-text-muted)]">
                                Problems Solved
                            </div>
                        </div>
                    </div>
                </div>
            </div>

            {/* Empty State */}
            {pastContests.length === 0 && (
                <div className="card text-center py-16">
                    <div className="w-16 h-16 rounded-2xl bg-[var(--color-surface-hover)] flex items-center justify-center mx-auto mb-4">
                        <FolderOpen className="w-8 h-8 text-[var(--color-text-muted)]" />
                    </div>
                    <h2 className="text-xl font-semibold mb-2">No contests yet</h2>
                    <p className="text-[var(--color-text-muted)] mb-6">
                        Start your first contest and track your progress here
                    </p>
                    <Link to="/contest/create" className="btn btn-primary">
                        Create Your First Contest
                    </Link>
                </div>
            )}

            {/* Contest List */}
            {pastContests.length > 0 && (
                <div className="space-y-4">
                    {pastContests
                        .sort((a, b) => new Date(b.started_at).getTime() - new Date(a.started_at).getTime())
                        .map((contest) => (
                            <ContestCard key={contest.id} contest={contest} />
                        ))}
                </div>
            )}
        </div>
    );
}

// Contest Card Component
function ContestCard({ contest }: { contest: Contest }) {
    const completedCount = contest.problems.filter(p => p.is_completed).length;
    const totalCount = contest.problems.length;
    const completionRate = Math.round((completedCount / totalCount) * 100);

    const isCompleted = contest.status === 'completed';

    return (
        <Link
            to={`/contest/${contest.id}`}
            className="card flex items-center gap-4 group"
        >
            {/* Status Icon */}
            <div className={clsx(
                'w-12 h-12 rounded-xl flex items-center justify-center',
                isCompleted
                    ? 'bg-[var(--color-success)]/15'
                    : 'bg-[var(--color-error)]/15'
            )}>
                {isCompleted ? (
                    <Trophy className="w-6 h-6 text-[var(--color-success)]" />
                ) : (
                    <XCircle className="w-6 h-6 text-[var(--color-error)]" />
                )}
            </div>

            {/* Contest Info */}
            <div className="flex-1 min-w-0">
                <div className="flex items-center gap-2 mb-1">
                    <span className={clsx(
                        'badge',
                        isCompleted ? 'badge-easy' : 'badge-hard'
                    )}>
                        {isCompleted ? 'Completed' : 'Abandoned'}
                    </span>
                    <span className="text-sm text-[var(--color-text-muted)]">
                        {formatDistanceToNow(new Date(contest.started_at), { addSuffix: true })}
                    </span>
                </div>
                <div className="flex items-center gap-4 text-sm text-[var(--color-text-muted)]">
                    <div className="flex items-center gap-1">
                        <Clock className="w-4 h-4" />
                        {contest.duration_minutes} min
                    </div>
                    <div className="flex items-center gap-1">
                        <CheckCircle2 className="w-4 h-4" />
                        {completedCount}/{totalCount} solved
                    </div>
                </div>
            </div>

            {/* Completion Rate */}
            <div className="text-right">
                <div className="text-2xl font-bold">{completionRate}%</div>
                <div className="text-xs text-[var(--color-text-muted)]">completion</div>
            </div>

            {/* Progress Ring */}
            <div className="relative w-10 h-10">
                <svg className="w-full h-full -rotate-90">
                    <circle
                        cx="20"
                        cy="20"
                        r="16"
                        fill="none"
                        stroke="var(--color-surface-hover)"
                        strokeWidth="4"
                    />
                    <circle
                        cx="20"
                        cy="20"
                        r="16"
                        fill="none"
                        stroke={isCompleted ? 'var(--color-success)' : 'var(--color-error)'}
                        strokeWidth="4"
                        strokeLinecap="round"
                        strokeDasharray="100"
                        strokeDashoffset={100 - completionRate}
                    />
                </svg>
            </div>

            {/* Arrow */}
            <ChevronRight className="w-5 h-5 text-[var(--color-text-muted)] group-hover:text-white transition-colors" />
        </Link>
    );
}
