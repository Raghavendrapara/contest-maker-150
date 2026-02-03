import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { contestApi } from '@/services/api';
import {
    Clock,
    Hash,
    Zap,
    Loader2,
    ChevronDown,
    ChevronUp,
    Info
} from 'lucide-react';
import clsx from 'clsx';

const PROBLEM_COUNTS = [3, 5, 7, 10];
const DURATIONS = [30, 60, 90, 120]; // in minutes

export default function CreateContest() {
    const navigate = useNavigate();
    const queryClient = useQueryClient();

    const [problemCount, setProblemCount] = useState(5);
    const [duration, setDuration] = useState(60);
    const [showAdvanced, setShowAdvanced] = useState(false);
    const [error, setError] = useState('');

    const createMutation = useMutation({
        mutationFn: () => contestApi.create({
            problem_count: problemCount,
            duration_minutes: duration,
        }),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['active-contest'] });
            navigate('/contest/active');
        },
        onError: (err: any) => {
            if (err.response?.data?.error) {
                setError(err.response.data.error);
            } else {
                setError('Failed to create contest. Please try again.');
            }
        },
    });

    const handleSubmit = (e: React.FormEvent) => {
        e.preventDefault();
        setError('');
        createMutation.mutate();
    };

    // Calculate expected difficulty distribution
    const getDistribution = (count: number) => {
        if (count <= 3) return { easy: Math.ceil(count * 0.5), medium: Math.floor(count * 0.5), hard: 0 };
        if (count <= 5) return { easy: Math.ceil(count * 0.4), medium: Math.floor(count * 0.4), hard: count - Math.ceil(count * 0.4) - Math.floor(count * 0.4) };
        return { easy: Math.ceil(count * 0.3), medium: Math.floor(count * 0.5), hard: count - Math.ceil(count * 0.3) - Math.floor(count * 0.5) };
    };

    const distribution = getDistribution(problemCount);

    return (
        <div className="max-w-2xl mx-auto animate-fadeIn">
            {/* Header */}
            <div className="mb-8">
                <h1 className="text-3xl font-bold mb-2">Create New Contest</h1>
                <p className="text-[var(--color-text-muted)]">
                    Customize your practice session with problems from NeetCode 150
                </p>
            </div>

            {/* Error message */}
            {error && (
                <div className="mb-6 p-4 rounded-lg bg-[var(--color-error)]/10 border border-[var(--color-error)]/20 text-[var(--color-error)] text-sm">
                    {error}
                </div>
            )}

            <form onSubmit={handleSubmit} className="space-y-8">
                {/* Problem Count */}
                <div className="card">
                    <div className="flex items-center gap-3 mb-6">
                        <div className="w-10 h-10 rounded-xl bg-gradient-to-br from-[var(--color-primary)] to-[var(--color-secondary)] flex items-center justify-center">
                            <Hash className="w-5 h-5 text-white" />
                        </div>
                        <div>
                            <h2 className="font-semibold">Number of Problems</h2>
                            <p className="text-sm text-[var(--color-text-muted)]">
                                Choose how many problems to include
                            </p>
                        </div>
                    </div>

                    <div className="grid grid-cols-4 gap-3">
                        {PROBLEM_COUNTS.map((count) => (
                            <button
                                key={count}
                                type="button"
                                onClick={() => setProblemCount(count)}
                                className={clsx(
                                    'py-4 px-6 rounded-xl text-center font-semibold transition-all',
                                    problemCount === count
                                        ? 'bg-gradient-to-r from-[var(--color-primary)] to-[var(--color-secondary)] text-white shadow-lg'
                                        : 'bg-[var(--color-surface-hover)] hover:bg-[var(--color-border)]'
                                )}
                            >
                                {count}
                            </button>
                        ))}
                    </div>

                    {/* Distribution Preview */}
                    <div className="mt-6 p-4 rounded-lg bg-[var(--color-background)] border border-[var(--color-border)]">
                        <div className="flex items-center gap-2 mb-3">
                            <Info className="w-4 h-4 text-[var(--color-text-muted)]" />
                            <span className="text-sm text-[var(--color-text-muted)]">
                                Expected difficulty distribution
                            </span>
                        </div>
                        <div className="flex items-center gap-4">
                            <div className="flex items-center gap-2">
                                <span className="w-3 h-3 rounded-full bg-[var(--color-easy)]" />
                                <span className="text-sm">{distribution.easy} Easy</span>
                            </div>
                            <div className="flex items-center gap-2">
                                <span className="w-3 h-3 rounded-full bg-[var(--color-medium)]" />
                                <span className="text-sm">{distribution.medium} Medium</span>
                            </div>
                            <div className="flex items-center gap-2">
                                <span className="w-3 h-3 rounded-full bg-[var(--color-hard)]" />
                                <span className="text-sm">{distribution.hard} Hard</span>
                            </div>
                        </div>
                    </div>
                </div>

                {/* Duration */}
                <div className="card">
                    <div className="flex items-center gap-3 mb-6">
                        <div className="w-10 h-10 rounded-xl bg-[var(--color-surface-hover)] flex items-center justify-center">
                            <Clock className="w-5 h-5 text-[var(--color-primary)]" />
                        </div>
                        <div>
                            <h2 className="font-semibold">Time Limit</h2>
                            <p className="text-sm text-[var(--color-text-muted)]">
                                Set the contest duration
                            </p>
                        </div>
                    </div>

                    <div className="grid grid-cols-4 gap-3">
                        {DURATIONS.map((mins) => (
                            <button
                                key={mins}
                                type="button"
                                onClick={() => setDuration(mins)}
                                className={clsx(
                                    'py-4 px-6 rounded-xl text-center font-semibold transition-all',
                                    duration === mins
                                        ? 'bg-gradient-to-r from-[var(--color-primary)] to-[var(--color-secondary)] text-white shadow-lg'
                                        : 'bg-[var(--color-surface-hover)] hover:bg-[var(--color-border)]'
                                )}
                            >
                                {mins} min
                            </button>
                        ))}
                    </div>
                </div>

                {/* Advanced Options (Optional) */}
                <button
                    type="button"
                    onClick={() => setShowAdvanced(!showAdvanced)}
                    className="flex items-center gap-2 text-[var(--color-text-muted)] hover:text-white transition-colors"
                >
                    {showAdvanced ? <ChevronUp className="w-4 h-4" /> : <ChevronDown className="w-4 h-4" />}
                    <span className="text-sm">Advanced options</span>
                </button>

                {showAdvanced && (
                    <div className="card bg-[var(--color-background)]">
                        <div className="flex items-center gap-2 mb-4">
                            <input
                                type="number"
                                min="1"
                                max="150"
                                value={problemCount}
                                onChange={(e) => setProblemCount(Math.min(150, Math.max(1, parseInt(e.target.value) || 1)))}
                                className="input w-24 text-center"
                            />
                            <span className="text-[var(--color-text-muted)]">problems</span>
                        </div>
                        <div className="flex items-center gap-2">
                            <input
                                type="number"
                                min="5"
                                max="300"
                                value={duration}
                                onChange={(e) => setDuration(Math.min(300, Math.max(5, parseInt(e.target.value) || 5)))}
                                className="input w-24 text-center"
                            />
                            <span className="text-[var(--color-text-muted)]">minutes</span>
                        </div>
                    </div>
                )}

                {/* Submit Button */}
                <button
                    type="submit"
                    disabled={createMutation.isPending}
                    className="btn btn-primary w-full py-4 text-lg"
                >
                    {createMutation.isPending ? (
                        <>
                            <Loader2 className="w-5 h-5 animate-spin" />
                            Creating Contest...
                        </>
                    ) : (
                        <>
                            <Zap className="w-5 h-5" />
                            Start Contest
                        </>
                    )}
                </button>

                {/* Info */}
                <p className="text-center text-sm text-[var(--color-text-muted)]">
                    Problems will be selected from your unsolved NeetCode 150 questions
                    with increasing difficulty.
                </p>
            </form>
        </div>
    );
}
