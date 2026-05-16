import React, { useState, useEffect } from 'react';
import { useSearchParams } from 'react-router-dom';
import { format, addDays, subDays, parseISO } from 'date-fns';
import { ChevronLeft, ChevronRight, Save, Calendar as CalendarIcon } from 'lucide-react';
import api from '../lib/api';

interface Entry {
    id: number;
    title: string;
    content: string;
    day: string;
    updated_at: string;
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
            } else {
                setContent('');
                setTitle('');
                setLastSaved(null);
            }
        } catch (err: any) {
            if (err.response?.status === 404) {
                setContent('');
                setTitle('');
                setLastSaved(null);
            }
        } finally {
            setLoading(false);
        }
    };

    const handleSave = async () => {
        setSaving(true);
        try {
            await api.post('/entries', {
                date: dateStr,
                content,
                title,
            });
            setLastSaved(new Date());
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
