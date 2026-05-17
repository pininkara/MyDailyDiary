import React, { useState, useEffect } from 'react';
import { useSearchParams } from 'react-router-dom';
import { format, addDays, subDays, parseISO } from 'date-fns';
import { ChevronLeft, ChevronRight, Save } from 'lucide-react';
import api from '../lib/api';

const ratingLabels = [1, 2, 3, 4, 5];

function RatingControl({
    label,
    value,
    onChange,
    leftEmoji,
    rightEmoji,
}: {
    label: string;
    value: number;
    onChange: (value: number) => void;
    leftEmoji: string;
    rightEmoji: string;
}) {
    return (
        <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-4">
            <div className="flex items-center justify-between">
                <span className="text-sm font-medium text-gray-700 dark:text-gray-200">{label}</span>
                <span className="text-xs text-gray-400">{value}/5</span>
            </div>
            <div className="mt-3 flex items-center gap-2">
                <span className="shrink-0 select-none text-lg leading-none">{leftEmoji}</span>
                <div className="relative flex-1 rounded-xl bg-gray-100 dark:bg-gray-700 p-1 overflow-hidden h-14">
                    <div className="absolute inset-1 grid grid-cols-5 gap-1 pointer-events-none">
                        {ratingLabels.map((rating) => {
                            const active = value === rating;
                            return (
                                <div
                                    key={rating}
                                    className={`flex items-center justify-center rounded-lg text-sm font-semibold transition-all ${
                                        active
                                            ? 'bg-white dark:bg-gray-800 text-indigo-600 shadow-sm ring-1 ring-black/5 dark:ring-white/10'
                                            : 'text-gray-500 dark:text-gray-300'
                                    }`}
                                >
                                    {rating}
                                </div>
                            );
                        })}
                    </div>
                    <input
                        type="range"
                        min={1}
                        max={5}
                        step={1}
                        value={value}
                        onChange={(e) => onChange(Number(e.target.value))}
                        aria-label={label}
                        className="absolute inset-0 h-full w-full cursor-pointer opacity-0 touch-none"
                    />
                </div>
                <span className="shrink-0 select-none text-lg leading-none">{rightEmoji}</span>
            </div>
        </div>
    );
}

export default function Home() {
    const [searchParams, setSearchParams] = useSearchParams();
    const [date, setDate] = useState(() => {
        const dateParam = searchParams.get('date');
        if (dateParam) {
            const parsed = parseISO(dateParam);
            if (!isNaN(parsed.getTime())) {
                return parsed;
            }
        }
        return new Date();
    });
    const [content, setContent] = useState('');
    const [title, setTitle] = useState('');
    const [saving, setSaving] = useState(false);
    const [loading, setLoading] = useState(false);
    const [lastSaved, setLastSaved] = useState<Date | null>(null);
    const [editCount, setEditCount] = useState(0);
    const [mood, setMood] = useState(3);
    const [fulfillment, setFulfillment] = useState(3);

    const dateStr = format(date, 'yyyy-MM-dd');

    useEffect(() => {
        setSearchParams({ date: dateStr }, { replace: true });
        loadEntry();
    }, [dateStr]);

    const loadEntry = async () => {
        setLoading(true);
        try {
            const res = await api.get(`/entries/date/${dateStr}`);
            if (res.data) {
                setContent(res.data.content || '');
                setTitle(res.data.title || '');
                setLastSaved(res.data.updated_at ? parseISO(res.data.updated_at) : null);
                setEditCount(res.data.edit_count || 0);
                setMood(res.data.mood ?? 0);
                setFulfillment(res.data.fulfillment ?? 0);
            } else {
                setContent('');
                setTitle('');
                setLastSaved(null);
                setEditCount(0);
                setMood(3);
                setFulfillment(3);
            }
        } catch (err: any) {
            if (err.response?.status === 404) {
                setContent('');
                setTitle('');
                setLastSaved(null);
                setEditCount(0);
                setMood(3);
                setFulfillment(3);
            }
        } finally {
            setLoading(false);
        }
    };

    const handleSave = async () => {
        setSaving(true);
        try {
            const res = await api.post('/entries', {
                date: dateStr,
                content,
                title,
                mood,
                fulfillment,
            });
            if (res.data) {
                setLastSaved(res.data.updated_at ? parseISO(res.data.updated_at) : new Date());
                setEditCount(res.data.edit_count || 0);
                setMood(res.data.mood ?? mood);
                setFulfillment(res.data.fulfillment ?? fulfillment);
            } else {
                setLastSaved(new Date());
            }
        } catch (err) {
            console.error('Failed to save', err);
        } finally {
            setSaving(false);
        }
    };

    // Auto-save effect (debounce could be added)
    useEffect(() => {
        const timer = setTimeout(() => {
            if (content) {
                // handleSave(); // Optional: auto-save
            }
        }, 2000);
        return () => clearTimeout(timer);
    }, [content, title]);

    return (
        <div className="space-y-6">
            {/* Header / Date Nav */}
            <div className="flex flex-col md:flex-row md:items-center justify-between gap-4">
                <div className="flex items-center gap-4">
                    <button
                        onClick={() => setDate(subDays(date, 1))}
                        className="p-2 hover:bg-gray-100 dark:hover:bg-gray-800 rounded-full"
                    >
                        <ChevronLeft className="w-5 h-5" />
                    </button>

                    <div className="flex flex-col items-center">
                        <h2 className="text-2xl font-bold text-gray-900 dark:text-white">
                            {format(date, 'MMMM d, yyyy')}
                        </h2>
                        <span className="text-sm text-gray-500">{format(date, 'EEEE')}</span>
                    </div>

                    <button
                        onClick={() => setDate(addDays(date, 1))}
                        className="p-2 hover:bg-gray-100 dark:hover:bg-gray-800 rounded-full"
                    >
                        <ChevronRight className="w-5 h-5" />
                    </button>

                    <button
                        onClick={() => setDate(new Date())}
                        className="ml-2 text-sm text-indigo-600 hover:underline"
                    >
                        Today
                    </button>
                </div>

                <div className="flex items-center gap-3">
                    <span className="text-xs text-gray-400">Edits {editCount}</span>
                    {lastSaved && (
                        <span className="text-xs text-gray-400">
                            Saved {format(lastSaved, 'HH:mm')}
                        </span>
                    )}
                    <button
                        onClick={handleSave}
                        disabled={saving}
                        className="flex items-center gap-2 px-4 py-2 bg-indigo-600 text-white rounded-lg hover:bg-indigo-700 disabled:opacity-50 transition-colors"
                    >
                        <Save className="w-4 h-4" />
                        {saving ? 'Saving...' : 'Save'}
                    </button>
                </div>
            </div>

            {/* Mood & Fulfillment */}
            <div className="grid md:grid-cols-2 gap-4">
                <RatingControl label="Mood" value={mood} onChange={setMood} leftEmoji="😭" rightEmoji="🥰" />
                <RatingControl
                    label="Fulfillment"
                    value={fulfillment}
                    onChange={setFulfillment}
                    leftEmoji="😴"
                    rightEmoji="💪"
                />
            </div>

            {/* Editor */}
            <div className="bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-200 dark:border-gray-700 overflow-hidden min-h-[60vh] flex flex-col">
                <input
                    type="text"
                    value={title}
                    onChange={(e) => setTitle(e.target.value)}
                    placeholder="Title (optional)"
                    className="w-full px-6 py-4 text-xl font-semibold bg-transparent border-b border-gray-100 dark:border-gray-700 focus:outline-none dark:text-white"
                />
                <textarea
                    value={content}
                    onChange={(e) => setContent(e.target.value)}
                    placeholder="Write your thoughts..."
                    className="flex-1 w-full p-6 resize-none focus:outline-none bg-transparent text-lg leading-relaxed dark:text-gray-200"
                />
            </div>
        </div>
    );
}
