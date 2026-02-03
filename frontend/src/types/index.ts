// User types
export interface User {
    id: string;
    email: string;
    username: string;
    created_at: string;
}

export interface UserProgress {
    total_solved: number;
    easy_solved: number;
    medium_solved: number;
    hard_solved: number;
    topic_progress: Record<string, TopicStats>;
    contest_stats: ContestStats;
}

export interface TopicStats {
    total: number;
    solved: number;
}

export interface ContestStats {
    total_contests: number;
    completed_contests: number;
    abandoned_contests: number;
}

// Auth types
export interface AuthResponse {
    user: User;
    tokens: TokenPair;
}

export interface TokenPair {
    access_token: string;
    refresh_token: string;
    expires_at: string;
}

export interface LoginRequest {
    email: string;
    password: string;
}

export interface SignupRequest {
    email: string;
    username: string;
    password: string;
}

// Problem types
export type Difficulty = 'Easy' | 'Medium' | 'Hard';

export interface Problem {
    id: string;
    title: string;
    slug: string;
    difficulty: Difficulty;
    topics: string[];
    leetcode_url: string;
    neetcode_url: string;
}

export interface ProblemStats {
    total: number;
    by_difficulty: Record<Difficulty, number>;
    by_topic: Record<string, number>;
}

// Contest types
export type ContestStatus = 'active' | 'completed' | 'abandoned';

export interface Contest {
    id: string;
    duration_minutes: number;
    started_at: string;
    ended_at: string | null;
    status: ContestStatus;
    problems: ContestProblem[];
    time_remaining_seconds: number;
}

export interface ContestProblem {
    order: number;
    is_completed: boolean;
    problem: Problem;
}

export interface CreateContestRequest {
    problem_count: number;
    duration_minutes: number;
}

// API response types
export interface ApiError {
    error: string;
    details?: string;
}

export interface ProblemsResponse {
    problems: Problem[];
    count: number;
}

export interface ContestsResponse {
    contests: Contest[];
}
