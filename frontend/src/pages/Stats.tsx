import React, { useEffect, useState } from 'react';
import api from '../lib/api';

interface DayCell {
    date: string;
    count: number;
    level: number;
}

interface StatsResponse {
    weeks: DayCell[][];
}

const levelClass = (level: number) => {
    if (level <= 0) return 'bg-gray-100 dark:bg-gray-700';
    if (level === 1) return 'bg-green-200 dark:bg-green-900';
    if (level === 2) return 'bg-green-300 dark:bg-green-800';
    if (level === 3) return 'bg-green-500 dark:bg-green-700';
    return 'bg-green-700 dark:bg-green-600';
};

export default function Stats() {
    const [stats, setStats] = useState<StatsResponse | null>(null);
    const [loading, setLoading] = useState(false);
    const [hovered, setHovered] = useState<DayCell | null>(null);

    useEffect(() => {
        const loadStats = async () => {
            setLoading(true);
            try {
                const res = await api.get('/stats');
                setStats(res.data || null);
            } catch (err) {
                console.error('Failed to load stats', err);
            } finally {
                setLoading(false);
            }
        };
        loadStats();
    }, []);

    const totalEdits = stats?.weeks.flat().reduce((sum, day) => sum + day.count, 0) || 0;

    return (
        <div className="space-y-6">
            <div className="flex items-center justify-between">
                <h2 className="text-2xl font-bold text-gray-900 dark:text-white">Stats</h2>
                {loading && <span className="text-xs text-gray-400">Loading...</span>}
            </div>

            {stats && (
                <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-6 space-y-6">
                    <div>
                        <div className="flex flex-col md:flex-row md:items-center md:justify-between gap-2 mb-3">
                            <div>
                                <div className="text-sm text-gray-500">Last 12 months (by edits)</div>
                                <div className="text-xs text-gray-400">{totalEdits} edits in the last year</div>
                            </div>
                            <div className="h-6 text-xs text-gray-500 dark:text-gray-400">
                                {hovered ? `${hovered.count} edit${hovered.count === 1 ? '' : 's'} on ${hovered.date}` : 'Hover a day to see details'}
                            </div>
                        </div>
                        <div className="overflow-x-auto">
                            <div className="grid grid-flow-col auto-cols-max gap-2">
                                {stats.weeks.map((week, weekIndex) => (
                                    <div key={`week-${weekIndex}`} className="grid grid-rows-7 gap-1">
                                        {week.map((day) => (
                                            <div
                                                key={day.date}
                                                title={`${day.date} · ${day.count}`}
                                                onMouseEnter={() => setHovered(day)}
                                                onMouseLeave={() => setHovered(null)}
                                                className={`w-3 h-3 rounded ${levelClass(day.level)}`}
                                            />
                                        ))}
                                    </div>
                                ))}
                            </div>
                        </div>
                        <div className="mt-4 flex items-center gap-2 text-xs text-gray-500">
                            <span>Less</span>
                            <div className={`w-3 h-3 rounded ${levelClass(0)}`} />
                            <div className={`w-3 h-3 rounded ${levelClass(1)}`} />
                            <div className={`w-3 h-3 rounded ${levelClass(2)}`} />
                            <div className={`w-3 h-3 rounded ${levelClass(3)}`} />
                            <div className={`w-3 h-3 rounded ${levelClass(4)}`} />
                            <span>More</span>
                        </div>
                    </div>
                </div>
            )}

            {!loading && !stats && (
                <div className="text-center py-12 text-gray-500">No stats available.</div>
            )}
        </div>
    );
}
