import React, { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { format, parseISO } from 'date-fns';
import api from '../lib/api';
import { Calendar, Clock, ChevronDown, ChevronUp } from 'lucide-react';

interface HistoryEntry {
    id: number;
    title: string;
    content: string;
    created_at: string;
    updated_at: string;
    day: string;
    snippet: string;
}

export default function History() {
    const [entries, setEntries] = useState<HistoryEntry[]>([]);
    const [loading, setLoading] = useState(false);
    const [offset, setOffset] = useState(0);
    const [hasMore, setHasMore] = useState(true);
    const [expanded, setExpanded] = useState<Set<number>>(new Set());
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

    const loadEntries = async (reset = false) => {
        if (loading) return;
        setLoading(true);
        try {
            const currentOffset = reset ? 0 : offset;
            const res = await api.get(`/entries?limit=${limit}&offset=${currentOffset}`);
            const newEntries = res.data;

            if (reset) {
                setEntries(newEntries);
            } else {
                setEntries(prev => [...prev, ...newEntries]);
            }

            if (newEntries.length < limit) {
                setHasMore(false);
            }
            setOffset(currentOffset + limit);
        } catch (err) {
            console.error(err);
        } finally {
            setLoading(false);
        }
    };

    useEffect(() => {
        loadEntries(true);
    }, []);

    return (
        <div className="space-y-6">
            <h2 className="text-2xl font-bold text-gray-900 dark:text-white">History</h2>

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
                                    {format(parseISO(entry.updated_at), 'HH:mm')}
                                </span>
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
                        </div>
                    </Link>
                ))}
            </div>

            {hasMore && (
                <div className="flex justify-center pt-4">
                    <button
                        onClick={() => loadEntries(false)}
                        disabled={loading}
                        className="px-6 py-2 bg-white dark:bg-gray-800 border border-gray-300 dark:border-gray-600 rounded-lg text-sm font-medium text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-gray-700 disabled:opacity-50"
                    >
                        {loading ? 'Loading...' : 'Load More'}
                    </button>
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
