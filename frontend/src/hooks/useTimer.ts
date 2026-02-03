import { useState, useEffect, useCallback, useRef } from 'react';

interface UseTimerOptions {
    initialSeconds: number;
    onExpire?: () => void;
    autoStart?: boolean;
}

interface UseTimerReturn {
    seconds: number;
    minutes: number;
    hours: number;
    totalSeconds: number;
    isRunning: boolean;
    isExpired: boolean;
    progress: number; // 0-100 percentage remaining
    start: () => void;
    pause: () => void;
    reset: (newSeconds?: number) => void;
    formattedTime: string;
}

export function useTimer({
    initialSeconds,
    onExpire,
    autoStart = true,
}: UseTimerOptions): UseTimerReturn {
    const [totalSeconds, setTotalSeconds] = useState(initialSeconds);
    const [isRunning, setIsRunning] = useState(autoStart);
    const [isExpired, setIsExpired] = useState(false);
    const initialSecondsRef = useRef(initialSeconds);
    const onExpireRef = useRef(onExpire);

    // Update refs when props change
    useEffect(() => {
        onExpireRef.current = onExpire;
    }, [onExpire]);

    useEffect(() => {
        initialSecondsRef.current = initialSeconds;
    }, [initialSeconds]);

    // Timer logic
    useEffect(() => {
        if (!isRunning || isExpired) return;

        const interval = setInterval(() => {
            setTotalSeconds((prev) => {
                if (prev <= 1) {
                    setIsRunning(false);
                    setIsExpired(true);
                    onExpireRef.current?.();
                    return 0;
                }
                return prev - 1;
            });
        }, 1000);

        return () => clearInterval(interval);
    }, [isRunning, isExpired]);

    const start = useCallback(() => {
        if (!isExpired) {
            setIsRunning(true);
        }
    }, [isExpired]);

    const pause = useCallback(() => {
        setIsRunning(false);
    }, []);

    const reset = useCallback((newSeconds?: number) => {
        const seconds = newSeconds ?? initialSecondsRef.current;
        setTotalSeconds(seconds);
        setIsExpired(false);
        setIsRunning(false);
    }, []);

    // Calculate derived values
    const hours = Math.floor(totalSeconds / 3600);
    const minutes = Math.floor((totalSeconds % 3600) / 60);
    const seconds = totalSeconds % 60;
    const progress = (totalSeconds / initialSecondsRef.current) * 100;

    // Format time string
    const formattedTime = hours > 0
        ? `${hours.toString().padStart(2, '0')}:${minutes.toString().padStart(2, '0')}:${seconds.toString().padStart(2, '0')}`
        : `${minutes.toString().padStart(2, '0')}:${seconds.toString().padStart(2, '0')}`;

    return {
        seconds,
        minutes,
        hours,
        totalSeconds,
        isRunning,
        isExpired,
        progress,
        start,
        pause,
        reset,
        formattedTime,
    };
}
