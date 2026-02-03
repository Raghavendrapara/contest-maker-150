import { useEffect } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { contestApi } from '@/services/api';
import { useTimer } from '@/hooks/useTimer';
import type { Contest, ContestProblem } from '@/types';
import {
    Clock,
    ExternalLink,
    CheckCircle2,
    Circle,
    AlertTriangle,
    Trophy,
    XCircle,
    Loader2
} from 'lucide-react';
import clsx from 'clsx';

export default function ActiveContest() {
    const navigate = useNavigate();
    const { id } = useParams<{ id: string }>();
    const queryClient = useQueryClient();

    // Fetch active contest or specific contest by ID
    const { data, isLoading } = useQuery<{ contest: Contest | null } | Contest>({
        queryKey: id ? ['contest', id] : ['active-contest'],
        queryFn: () => id ? contestApi.getById(id) : contestApi.getActive(),
        refetchInterval: 30000, // Refresh every 30 seconds
    });

    // Normalize data (API returns { contest: Contest } for active, Contest for specific)
    const contest = data ? ('contest' in data ? data.contest : data) : null;

    // Timer hook
    const {
        formattedTime,
        isExpired,
        progress,
        totalSeconds,
    } = useTimer({
        initialSeconds: contest?.time_remaining_seconds ?? 0,
        autoStart: !!contest && contest.status === 'active',
        onExpire: () => {
            queryClient.invalidateQueries({ queryKey: ['active-contest'] });
        },
    });

    // Mark problem complete mutation
    const markCompleteMutation = useMutation({
        mutationFn: ({ problemId, isCompleted }: { problemId: string; isCompleted: boolean }) =>
            contestApi.markProblemComplete(contest!.id, problemId, isCompleted),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: id ? ['contest', id] : ['active-contest'] });
        },
    });

    // Complete contest mutation
    const completeMutation = useMutation({
        mutationFn: () => contestApi.complete(contest!.id),
        onSuccess: () => {
            navigate('/dashboard');
            queryClient.invalidateQueries({ queryKey: ['active-contest'] });
            queryClient.invalidateQueries({ queryKey: ['user-progress'] });
        },
    });

    // Abandon contest mutation  
    const abandonMutation = useMutation({
        mutationFn: () => contestApi.abandon(contest!.id),
        onSuccess: () => {
            navigate('/dashboard');
            queryClient.invalidateQueries({ queryKey: ['active-contest'] });
        },
    });

    // Redirect if no active contest
    useEffect(() => {
        if (!isLoading && !contest && !id) {
            navigate('/contest/create');
        }
    }, [isLoading, contest, id, navigate]);

    if (isLoading) {
        return (
            <div className="flex items-center justify-center min-h-[60vh]">
                <Loader2 className="w-8 h-8 animate-spin text-[var(--color-primary)]" />
            </div>
        );
    }

    if (!contest) {
        return null;
    }

    const completedCount = contest.problems.filter(p => p.is_completed).length;
    const totalCount = contest.problems.length;
    const isActive = contest.status === 'active' && !isExpired;

    // Timer color based on remaining time
    const getTimerColor = () => {
        if (isExpired || totalSeconds <= 0) return 'var(--color-error)';
        if (progress < 25) return 'var(--color-error)';
        if (progress < 50) return 'var(--color-warning)';
        return 'var(--color-success)';
    };

    return (
        <div className="max-w-4xl mx-auto animate-fadeIn">
            {/* Header with Timer */}
            <div className="glass rounded-2xl p-6 mb-8">
                <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
                    {/* Timer */}
                    <div className="flex items-center gap-4">
                        <div
                            className="relative w-20 h-20 flex items-center justify-center"
                            style={{ '--timer-color': getTimerColor() } as React.CSSProperties}
                        >
                            <svg className="w-full h-full -rotate-90">
                                <circle
                                    cx="40"
                                    cy="40"
                                    r="35"
                                    fill="none"
                                    stroke="var(--color-surface-hover)"
                                    strokeWidth="6"
                                />
                                <circle
                                    cx="40"
                                    cy="40"
                                    r="35"
                                    fill="none"
                                    stroke={getTimerColor()}
                                    strokeWidth="6"
                                    strokeLinecap="round"
                                    strokeDasharray="220"
                                    strokeDashoffset={220 - (220 * progress) / 100}
                                    className="transition-all duration-1000"
                                />
                            </svg>
                            <div className="absolute inset-0 flex items-center justify-center">
                                <Clock className="w-5 h-5" style={{ color: getTimerColor() }} />
                            </div>
                        </div>
                        <div>
                            <div
                                className="text-3xl font-bold font-mono"
                                style={{ color: getTimerColor() }}
                            >
                                {formattedTime}
                            </div>
                            <div className="text-sm text-[var(--color-text-muted)]">
                                {isExpired ? 'Time expired' : 'Time remaining'}
                            </div>
                        </div>
                    </div>

                    {/* Progress */}
                    <div className="text-right">
                        <div className="text-2xl font-bold">
                            {completedCount} / {totalCount}
                        </div>
                        <div className="text-sm text-[var(--color-text-muted)]">
                            problems completed
                        </div>
                    </div>
                </div>

                {/* Progress Bar */}
                <div className="mt-4 h-2 bg-[var(--color-surface-hover)] rounded-full overflow-hidden">
                    <div
                        className="h-full bg-gradient-to-r from-[var(--color-primary)] to-[var(--color-secondary)] rounded-full transition-all duration-300"
                        style={{ width: `${(completedCount / totalCount) * 100}%` }}
                    />
                </div>
            </div>

            {/* Time Expired Warning */}
            {isExpired && contest.status === 'active' && (
                <div className="mb-6 p-4 rounded-lg bg-[var(--color-warning)]/10 border border-[var(--color-warning)]/20 flex items-center gap-3">
                    <AlertTriangle className="w-5 h-5 text-[var(--color-warning)]" />
                    <span className="text-[var(--color-warning)]">
                        Time has expired! Complete or abandon the contest to proceed.
                    </span>
                </div>
            )}

            {/* Problems List */}
            <div className="space-y-4 mb-8">
                {contest.problems
                    .sort((a, b) => a.order - b.order)
                    .map((contestProblem, index) => (
                        <ProblemCard
                            key={contestProblem.problem.id}
                            contestProblem={contestProblem}
                            index={index}
                            isActive={isActive}
                            onToggle={(isCompleted) =>
                                markCompleteMutation.mutate({
                                    problemId: contestProblem.problem.id,
                                    isCompleted,
                                })
                            }
                            isLoading={markCompleteMutation.isPending}
                        />
                    ))}
            </div>

            {/* Action Buttons */}
            {isActive && (
                <div className="flex flex-col sm:flex-row gap-4">
                    <button
                        onClick={() => completeMutation.mutate()}
                        disabled={completeMutation.isPending}
                        className="btn btn-primary flex-1 py-4"
                    >
                        {completeMutation.isPending ? (
                            <Loader2 className="w-5 h-5 animate-spin" />
                        ) : (
                            <Trophy className="w-5 h-5" />
                        )}
                        Complete Contest
                    </button>
                    <button
                        onClick={() => {
                            if (confirm('Are you sure you want to abandon this contest?')) {
                                abandonMutation.mutate();
                            }
                        }}
                        disabled={abandonMutation.isPending}
                        className="btn btn-secondary py-4"
                    >
                        {abandonMutation.isPending ? (
                            <Loader2 className="w-5 h-5 animate-spin" />
                        ) : (
                            <XCircle className="w-5 h-5" />
                        )}
                        Abandon
                    </button>
                </div>
            )}

            {/* Contest Ended */}
            {contest.status !== 'active' && (
                <div className={clsx(
                    'p-6 rounded-xl text-center',
                    contest.status === 'completed'
                        ? 'bg-[var(--color-success)]/10 border border-[var(--color-success)]/20'
                        : 'bg-[var(--color-error)]/10 border border-[var(--color-error)]/20'
                )}>
                    <div className={clsx(
                        'text-xl font-bold mb-2',
                        contest.status === 'completed' ? 'text-[var(--color-success)]' : 'text-[var(--color-error)]'
                    )}>
                        {contest.status === 'completed' ? 'Contest Completed!' : 'Contest Abandoned'}
                    </div>
                    <div className="text-[var(--color-text-muted)]">
                        You completed {completedCount} out of {totalCount} problems
                    </div>
                </div>
            )}
        </div>
    );
}

// Problem Card Component
interface ProblemCardProps {
    contestProblem: ContestProblem;
    index: number;
    isActive: boolean;
    onToggle: (isCompleted: boolean) => void;
    isLoading: boolean;
}

function ProblemCard({ contestProblem, index, isActive, onToggle, isLoading }: ProblemCardProps) {
    const { problem, is_completed } = contestProblem;

    const difficultyClass = {
        Easy: 'badge-easy',
        Medium: 'badge-medium',
        Hard: 'badge-hard',
    }[problem.difficulty];

    return (
        <div
            className={clsx(
                'card flex items-center gap-4 transition-all',
                is_completed && 'bg-[var(--color-success)]/5 border-[var(--color-success)]/20'
            )}
        >
            {/* Completion Toggle */}
            <button
                onClick={() => isActive && onToggle(!is_completed)}
                disabled={!isActive || isLoading}
                className={clsx(
                    'w-8 h-8 rounded-full flex items-center justify-center transition-all',
                    is_completed
                        ? 'bg-[var(--color-success)] text-white'
                        : 'bg-[var(--color-surface-hover)] text-[var(--color-text-muted)] hover:text-white',
                    !isActive && 'cursor-not-allowed opacity-50'
                )}
            >
                {is_completed ? (
                    <CheckCircle2 className="w-5 h-5" />
                ) : (
                    <Circle className="w-5 h-5" />
                )}
            </button>

            {/* Problem Number */}
            <div className="w-8 h-8 rounded-lg bg-[var(--color-surface-hover)] flex items-center justify-center font-semibold text-sm">
                {index + 1}
            </div>

            {/* Problem Info */}
            <div className="flex-1 min-w-0">
                <h3 className={clsx(
                    'font-medium truncate',
                    is_completed && 'line-through text-[var(--color-text-muted)]'
                )}>
                    {problem.title}
                </h3>
                <div className="flex items-center gap-2 mt-1">
                    <span className={clsx('badge', difficultyClass)}>
                        {problem.difficulty}
                    </span>
                    <span className="text-xs text-[var(--color-text-muted)]">
                        {problem.topics[0]}
                    </span>
                </div>
            </div>

            {/* External Links */}
            <div className="flex items-center gap-2">
                <a
                    href={problem.leetcode_url}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="btn btn-ghost p-2"
                    title="Open in LeetCode"
                >
                    <ExternalLink className="w-4 h-4" />
                </a>
            </div>
        </div>
    );
}
