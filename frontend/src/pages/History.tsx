import React, { useCallback, useEffect, useRef, useState } from 'react';
import { Link } from 'react-router-dom';
import { format, parseISO } from 'date-fns';
import api from '../lib/api';
import { Calendar, Clock, ChevronDown, ChevronUp, Edit3, Heart, BatteryFull, Loader2 } from 'lucide-react';
import { getAmbientWeatherEmojis, getBaseWeatherEmoji, countContentUnits } from '../lib/utils';

interface HistoryEntry {
    id: number;
    title: string;
    content: string;
    created_at: string;
    updated_at: string;
    day: string;
    snippet: string;
    edit_count: number;
    mood: number;
    fulfillment: number;
    base_weather: string;
    ambient_weathers: string[];
}

export default function History() {
    const [entries, setEntries] = useState<HistoryEntry[]>([]);
    const [loading, setLoading] = useState(false);
    const [hasMore, setHasMore] = useState(true);
    const [expanded, setExpanded] = useState<Set<number>>(new Set());
    const [filterFrom, setFilterFrom] = useState('');
    const [filterTo, setFilterTo] = useState('');
    const [appliedFrom, setAppliedFrom] = useState('');
    const [appliedTo, setAppliedTo] = useState('');
    const sentinelRef = useRef<HTMLDivElement | null>(null);
    const offsetRef = useRef(0);
    const loadingRef = useRef(false);
    const limit = 20;

    const toggleExpand = (id: number, e: React.MouseEvent) => {
        e.preventDefault();
        e.stopPropagation();
        setExpanded(prev => {
            const next = new Set(prev);
            if (next.has(id)) {
                next.delete(id);
            } else {
                next.add(id);
            }
            return next;
        });
    };

    const loadEntries = useCallback(async (reset = false) => {
        if (loadingRef.current) return;

        loadingRef.current = true;
        setLoading(true);
        try {
            const currentOffset = reset ? 0 : offsetRef.current;
            const params = new URLSearchParams({
                limit: String(limit),
                offset: String(currentOffset),
            });
            if (appliedFrom && appliedTo) {
                params.set('from', appliedFrom);
                params.set('to', appliedTo);
            }
            const res = await api.get(`/entries?${params.toString()}`);
            const newEntries = res.data;

            if (reset) {
                setEntries(newEntries);
            } else {
                setEntries((current) => {
                    const combined = [...current, ...newEntries];
                    return combined.filter(
                        (entry, index, all) => all.findIndex((item) => item.id === entry.id) === index
                    );
                });
            }

            setHasMore(newEntries.length === limit);
            offsetRef.current = currentOffset + newEntries.length;
        } catch (err) {
            console.error(err);
            setHasMore(false);
        } finally {
            loadingRef.current = false;
            setLoading(false);
        }
    }, [appliedFrom, appliedTo]);

    useEffect(() => {
        void loadEntries(true);
    }, [loadEntries]);

    useEffect(() => {
        const sentinel = sentinelRef.current;
        if (!sentinel || !hasMore || loading) return;

        const observer = new IntersectionObserver(
            (observations) => {
                if (observations[0]?.isIntersecting) {
                    void loadEntries();
                }
            },
            { rootMargin: '240px 0px' }
        );
        observer.observe(sentinel);
        return () => observer.disconnect();
    }, [hasMore, loadEntries, loading]);

    const applyRange = () => {
        if (!filterFrom || !filterTo) return;
        setHasMore(true);
        setAppliedFrom(filterFrom);
        setAppliedTo(filterTo);
    };

    const clearRange = () => {
        setFilterFrom('');
        setFilterTo('');
        setHasMore(true);
        setAppliedFrom('');
        setAppliedTo('');
    };

    return (
        <div className="space-y-6">
            <h2 className="text-2xl font-bold text-gray-900 dark:text-white">History</h2>

            <div className="flex flex-col md:flex-row md:items-end gap-3 bg-white dark:bg-gray-800 p-4 rounded-xl border border-gray-200 dark:border-gray-700">
                <div className="flex flex-col">
                    <label className="text-xs text-gray-500 mb-1">From</label>
                    <input
                        type="date"
                        value={filterFrom}
                        onChange={(e) => setFilterFrom(e.target.value)}
                        className="px-3 py-2 rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-900 text-sm dark:text-white"
                    />
                </div>
                <div className="flex flex-col">
                    <label className="text-xs text-gray-500 mb-1">To</label>
                    <input
                        type="date"
                        value={filterTo}
                        onChange={(e) => setFilterTo(e.target.value)}
                        className="px-3 py-2 rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-900 text-sm dark:text-white"
                    />
                </div>
                <div className="flex gap-2">
                    <button
                        onClick={applyRange}
                        disabled={!filterFrom || !filterTo || loading}
                        className="px-4 py-2 bg-indigo-600 text-white rounded-lg text-sm font-medium hover:bg-indigo-700 disabled:opacity-50"
                    >
                        Apply
                    </button>
                    <button
                        onClick={clearRange}
                        disabled={loading || (!appliedFrom && !appliedTo && !filterFrom && !filterTo)}
                        className="px-4 py-2 bg-white dark:bg-gray-900 border border-gray-300 dark:border-gray-600 rounded-lg text-sm font-medium text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-gray-800 disabled:opacity-50"
                    >
                        Clear
                    </button>
                </div>
            </div>

            <div className="space-y-4">
                {entries.map((entry) => (
                    <Link
                        key={entry.id}
                        to={`/?date=${entry.day}`}
                        className="block bg-white dark:bg-gray-800 p-6 rounded-xl shadow-sm border border-gray-200 dark:border-gray-700 hover:border-indigo-500 dark:hover:border-indigo-500 transition-colors"
                    >
                        <div className="flex justify-between items-start mb-2">
                            <h3 className="text-lg font-semibold text-gray-900 dark:text-white">
                                {entry.title || 'Untitled'}
                            </h3>
                            <div className="flex items-center text-sm text-gray-500 gap-4">
                                <span className="flex items-center gap-1">
                                    <Calendar className="w-4 h-4" />
                                    {format(parseISO(entry.day), 'MMM d, yyyy')}
                                </span>
                                <span className="flex items-center gap-1">
                                    <Clock className="w-4 h-4" />
                                    {format(parseISO(entry.updated_at), 'MMM d, yyyy HH:mm')}
                                </span>
                                {(entry.base_weather || entry.ambient_weathers?.length > 0) && (
                                    <span className="flex items-center gap-1">
                                        <span className="emoji-font text-base leading-none">
                                            {[getBaseWeatherEmoji(entry.base_weather), ...getAmbientWeatherEmojis(entry.ambient_weathers ?? [])]
                                                .filter(Boolean)
                                                .join(' ')}
                                        </span>
                                    </span>
                                )}
                                <span className="flex items-center gap-1">
                                    <Heart className="w-4 h-4" />
                                    {entry.mood ?? 0}
                                </span>
                                <span className="flex items-center gap-1">
                                    <BatteryFull className="w-4 h-4" />
                                    {entry.fulfillment ?? 0}
                                </span>
                                {/* moved edit_count to card footer */}
                            </div>
                        </div>
                        <div className="mt-2">
                            <p className={`text-gray-600 dark:text-gray-300 ${expanded.has(entry.id) ? 'whitespace-pre-wrap' : 'line-clamp-3'}`}>
                                {entry.content}
                            </p>
                            {entry.content.length > 100 && (
                                <button
                                    onClick={(e) => toggleExpand(entry.id, e)}
                                    className="mt-2 flex items-center gap-1 text-sm text-indigo-600 dark:text-indigo-400 hover:underline"
                                >
                                    {expanded.has(entry.id) ? (
                                        <>Show Less <ChevronUp className="w-4 h-4" /></>
                                    ) : (
                                        <>Show More <ChevronDown className="w-4 h-4" /></>
                                    )}
                                </button>
                            )}
                            <div className="mt-4 flex justify-end items-center gap-4 text-sm text-gray-500 dark:text-gray-400">
                                <div className="text-xs text-gray-500 dark:text-gray-400">{countContentUnits(entry.content)} words</div>
                                <span className="flex items-center gap-1">
                                    <Edit3 className="w-4 h-4" />
                                    {entry.edit_count}
                                </span>
                            </div>
                        </div>
                    </Link>
                ))}
            </div>

            <div ref={sentinelRef} className="h-1" aria-hidden="true" />

            {loading && (
                <div className="flex items-center justify-center gap-2 py-4 text-sm text-gray-500 dark:text-gray-400">
                    <Loader2 className="h-4 w-4 animate-spin" />
                    Loading...
                </div>
            )}

            {!loading && entries.length === 0 && (
                <div className="text-center py-12 text-gray-500">
                    No entries found. Start writing!
                </div>
            )}
        </div>
    );
}
