import React, { useEffect, useState } from 'react';
import { Cell, Legend, Pie, PieChart, ResponsiveContainer, Tooltip } from 'recharts';
import api from '../lib/api';

interface DayCell {
    date: string;
    count: number;
    words: number;
    level: number;
}

interface StatsResponse {
    weeks: DayCell[][];
    periods: Record<PeriodKey, PeriodStats>;
}

type PeriodKey = 'month' | 'week';

interface RatingDistributionItem {
    level: number;
    count: number;
    percent: number;
}

interface PeriodStats {
    entries: number;
    total_words: number;
    average_words: number;
    average_mood: number;
    average_fulfillment: number;
    mood_distribution: RatingDistributionItem[];
    fulfillment_distribution: RatingDistributionItem[];
}

const PERIODS: Array<{ key: PeriodKey; label: string }> = [
    { key: 'month', label: 'This Month' },
    { key: 'week', label: 'This Week' },
];

const CHART_COLORS = ['#AF767F', '#ABAAA2', '#7B92A1', '#DA9C51', '#8FA08D'];

const levelClass = (level: number) => {
    if (level <= 0) return 'bg-gray-100 dark:bg-gray-700';
    if (level === 1) return 'bg-green-200 dark:bg-green-900';
    if (level === 2) return 'bg-green-300 dark:bg-green-800';
    if (level === 3) return 'bg-green-500 dark:bg-green-700';
    return 'bg-green-700 dark:bg-green-600';
};

const formatNumber = (value: number) => new Intl.NumberFormat().format(Math.round(value));
const formatDecimal = (value: number) => value.toFixed(1);

const distributionHasData = (items: RatingDistributionItem[]) => items.some((item) => item.count > 0);

function RatingPie({ title, data }: { title: string; data: RatingDistributionItem[] }) {
    const chartData = data.map((item) => ({
        ...item,
        name: `${item.level}`,
    }));
    const legendData = chartData.map((item, index) => ({
        value: item.name,
        color: CHART_COLORS[index % CHART_COLORS.length],
    }));

    return (
        <div className="rounded-2xl border border-gray-200/80 dark:border-gray-700 bg-gradient-to-br from-white to-gray-50/70 dark:from-gray-800 dark:to-gray-800/80 p-5 shadow-sm">
            <h4 className="text-sm font-semibold text-gray-900 dark:text-white mb-3 tracking-wide">{title}</h4>
            {distributionHasData(data) ? (
                <div className="h-72 -mt-1">
                    <ResponsiveContainer width="100%" height="100%">
                        <PieChart>
                            <Pie
                                data={chartData}
                                dataKey="count"
                                nameKey="name"
                                cx="50%"
                                cy="40%"
                                outerRadius={96}
                                paddingAngle={1.5}
                                stroke="#ffffff"
                                strokeWidth={1.5}
                            >
                                {chartData.map((item, index) => (
                                    <Cell key={item.level} fill={CHART_COLORS[index % CHART_COLORS.length]} />
                                ))}
                            </Pie>
                            <Tooltip
                                contentStyle={{
                                    borderRadius: '12px',
                                    border: '1px solid rgba(148, 163, 184, 0.22)',
                                    background: 'rgba(255, 255, 255, 0.96)',
                                    boxShadow: '0 10px 30px rgba(15, 23, 42, 0.08)',
                                }}
                                labelStyle={{ color: '#334155', fontWeight: 600 }}
                                formatter={(value, _name, props) => [`${value} (${props.payload.percent}%)`, props.payload.name]}
                            />
                            <Legend
                                verticalAlign="bottom"
                                height={48}
                                iconType="circle"
                                content={() => (
                                    <div className="mt-2 flex flex-wrap items-center justify-center gap-x-4 gap-y-2 text-xs text-gray-500 dark:text-gray-300">
                                        {legendData.map((item) => (
                                            <div key={item.value} className="flex items-center gap-2">
                                                <span
                                                    className="inline-block h-2.5 w-2.5 rounded-full"
                                                    style={{ backgroundColor: item.color }}
                                                />
                                                <span>{item.value}</span>
                                            </div>
                                        ))}
                                    </div>
                                )}
                            />
                        </PieChart>
                    </ResponsiveContainer>
                </div>
            ) : (
                <div className="h-72 flex items-center justify-center text-sm text-gray-500 dark:text-gray-400">
                    No data for this period.
                </div>
            )}
        </div>
    );
}

export default function Stats() {
    const [stats, setStats] = useState<StatsResponse | null>(null);
    const [selectedPeriod, setSelectedPeriod] = useState<PeriodKey>('month');
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
    const selectedStats = stats?.periods[selectedPeriod];

    return (
        <div className="space-y-6">
            <div className="flex items-center justify-between">
                <h2 className="text-2xl font-bold text-gray-900 dark:text-white">Stats</h2>
                {loading && <span className="text-xs text-gray-400">Loading...</span>}
            </div>

            {stats && (
                <>
                    <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                        {PERIODS.map((period) => {
                            const periodStats = stats.periods[period.key];
                            const selected = selectedPeriod === period.key;
                            return (
                                <button
                                    key={period.key}
                                    type="button"
                                    aria-pressed={selected}
                                    onClick={() => setSelectedPeriod(period.key)}
                                    className={`text-left bg-white dark:bg-gray-800 rounded-xl border p-6 transition ${selected
                                        ? 'border-indigo-500 ring-2 ring-indigo-200 dark:ring-indigo-900'
                                        : 'border-gray-200 dark:border-gray-700 hover:border-indigo-300 dark:hover:border-indigo-700'
                                        }`}
                                >
                                    <div className="text-sm text-gray-500 dark:text-gray-400">{period.label}</div>
                                    <div className="mt-2 text-3xl font-bold text-gray-900 dark:text-white">
                                        {periodStats?.entries ?? 0}
                                    </div>
                                </button>
                            );
                        })}
                    </div>

                    {selectedStats && (
                        <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-6 space-y-6">
                            <div className="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-4 gap-4">
                                <div className="rounded-xl border border-gray-200 dark:border-gray-700 p-5">
                                    <div className="text-sm text-gray-500 dark:text-gray-400">Total Words</div>
                                    <div className="mt-2 text-3xl font-bold text-gray-900 dark:text-white">
                                        {formatNumber(selectedStats.total_words)}
                                    </div>
                                </div>
                                <div className="rounded-xl border border-gray-200 dark:border-gray-700 p-5">
                                    <div className="text-sm text-gray-500 dark:text-gray-400">Average Words</div>
                                    <div className="mt-2 text-3xl font-bold text-gray-900 dark:text-white">
                                        {formatNumber(selectedStats.average_words)}
                                    </div>
                                </div>
                                <div className="rounded-xl border border-gray-200 dark:border-gray-700 p-5">
                                    <div className="text-sm text-gray-500 dark:text-gray-400">Average Mood</div>
                                    <div className="mt-2 text-3xl font-bold text-gray-900 dark:text-white">
                                        {formatDecimal(selectedStats.average_mood)}
                                    </div>
                                </div>
                                <div className="rounded-xl border border-gray-200 dark:border-gray-700 p-5">
                                    <div className="text-sm text-gray-500 dark:text-gray-400">Average Fulfillment</div>
                                    <div className="mt-2 text-3xl font-bold text-gray-900 dark:text-white">
                                        {formatDecimal(selectedStats.average_fulfillment)}
                                    </div>
                                </div>
                            </div>

                            <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
                                <RatingPie title="Mood Distribution" data={selectedStats.mood_distribution} />
                                <RatingPie title="Fulfillment Distribution" data={selectedStats.fulfillment_distribution} />
                            </div>
                        </div>
                    )}

                    <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-6 space-y-6">
                        <div>
                            <div className="flex flex-col md:flex-row md:items-center md:justify-between gap-2 mb-3">
                                <div>
                                    <div className="text-sm text-gray-500">Last 12 months (by edits)</div>
                                    <div className="text-xs text-gray-400">{totalEdits} edits in the last year</div>
                                </div>
                                <div className="h-6 text-xs text-gray-500 dark:text-gray-400">
                                    {hovered
                                        ? `${hovered.count} edit${hovered.count === 1 ? '' : 's'}, ${hovered.words} word${hovered.words === 1 ? '' : 's'} on ${hovered.date}`
                                        : 'Hover a day to see details'}
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
                </>
            )}

            {!loading && !stats && (
                <div className="text-center py-12 text-gray-500">No stats available.</div>
            )}
        </div>
    );
}
