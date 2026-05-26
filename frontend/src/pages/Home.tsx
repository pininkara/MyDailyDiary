import React, { useState, useEffect } from 'react';
import { useSearchParams } from 'react-router-dom';
import { format, addDays, subDays, parseISO } from 'date-fns';
import { ChevronLeft, ChevronRight, Save, RefreshCw } from 'lucide-react';
import api from '../lib/api';
import {
    ambientWeatherOptions,
    baseWeatherOptions,
    cn,
    isAmbientWeatherValue,
    isBaseWeatherValue,
    type AmbientWeatherValue,
    type BaseWeatherValue,
    countContentUnits,
} from '../lib/utils';

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

function BaseWeatherControl({
    value,
    onChange,
}: {
    value: BaseWeatherValue | '';
    onChange: (value: BaseWeatherValue) => void;
}) {
    return (
        <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-4">
            <div className="flex items-center justify-between">
                <span className="text-sm font-medium text-gray-700 dark:text-gray-200">Base Weather</span>
                <span className="text-xs text-gray-400">1/1</span>
            </div>
            <div className="mt-3 rounded-xl bg-gray-100 dark:bg-gray-700 p-1">
                <div className="grid grid-cols-7 gap-1">
                    {baseWeatherOptions.map((option) => {
                        const active = value === option.value;
                        return (
                            <button
                                key={option.value}
                                type="button"
                                aria-label={option.label}
                                aria-pressed={active}
                                onClick={() => onChange(option.value)}
                                className={cn(
                                    'flex h-14 w-full items-center justify-center rounded-lg text-xl transition-all',
                                    active
                                        ? 'bg-white dark:bg-gray-800 shadow-sm ring-1 ring-black/5 dark:ring-white/10'
                                        : 'text-gray-500 hover:bg-white/70 dark:text-gray-300 dark:hover:bg-gray-600'
                                )}
                            >
                                {option.emoji}
                            </button>
                        );
                    })}
                </div>
            </div>
        </div>
    );
}

function AmbientWeatherControl({
    values,
    onToggle,
}: {
    values: AmbientWeatherValue[];
    onToggle: (value: AmbientWeatherValue) => void;
}) {
    return (
        <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-4">
            <div className="flex items-center justify-between">
                <span className="text-sm font-medium text-gray-700 dark:text-gray-200">Ambient Weather</span>
                <span className="text-xs text-gray-400">{values.length}/{ambientWeatherOptions.length}</span>
            </div>
            <div className="mt-3 grid grid-cols-6 gap-2 rounded-xl bg-gray-100 dark:bg-gray-700 p-2">
                {ambientWeatherOptions.map((option) => {
                    const active = values.includes(option.value);
                    return (
                        <button
                            key={option.value}
                            type="button"
                            aria-label={option.label}
                            aria-pressed={active}
                            onClick={() => onToggle(option.value)}
                            className={cn(
                                'flex min-h-16 flex-col items-center justify-center gap-1 rounded-lg px-1 py-2 transition-all',
                                active
                                    ? 'bg-white dark:bg-gray-800 shadow-sm ring-1 ring-black/5 dark:ring-white/10'
                                    : 'text-gray-500 hover:bg-white/70 dark:text-gray-300 dark:hover:bg-gray-600'
                            )}
                        >
                            <span className="text-xl leading-none">{option.emoji}</span>
                            <span className="text-[11px] leading-none text-gray-600 dark:text-gray-300 whitespace-nowrap">
                                {option.label}
                            </span>
                        </button>
                    );
                })}
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
    const [baseWeather, setBaseWeather] = useState<BaseWeatherValue | ''>('');
    const [ambientWeathers, setAmbientWeathers] = useState<AmbientWeatherValue[]>([]);

    const dateStr = format(date, 'yyyy-MM-dd');

    // Debounced display count to avoid running regex on every keystroke
    const [displayCount, setDisplayCount] = React.useState(0);
    React.useEffect(() => {
        const t = setTimeout(() => {
            setDisplayCount(countContentUnits(content));
        }, 350);
        return () => clearTimeout(t);
    }, [content]);

    const [toastMessage, setToastMessage] = useState('');

    const regenerateTitle = async () => {
        try {
            setToastMessage('正在重新生成标题');
            const res = await api.post('/generate-title-and-save', { content, date: dateStr });
            const entry = res.data;
            if (entry) {
                setTitle(entry.title || '');
                setLastSaved(entry.updated_at ? parseISO(entry.updated_at) : new Date());
                setEditCount(entry.edit_count || 0);
            }
        } catch (err) {
            console.error('generate+save title failed', err);
            setToastMessage('生成标题失败');
            setTimeout(() => setToastMessage(''), 2000);
            return;
        }
        setTimeout(() => setToastMessage(''), 1800);
    };

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
                setBaseWeather(isBaseWeatherValue(res.data.base_weather) ? res.data.base_weather : '');
                setAmbientWeathers(
                    Array.isArray(res.data.ambient_weathers)
                        ? res.data.ambient_weathers.filter(isAmbientWeatherValue)
                        : []
                );
            } else {
                setContent('');
                setTitle('');
                setLastSaved(null);
                setEditCount(0);
                setMood(3);
                setFulfillment(3);
                setBaseWeather('');
                setAmbientWeathers([]);
            }
        } catch (err: any) {
            if (err.response?.status === 404) {
                setContent('');
                setTitle('');
                setLastSaved(null);
                setEditCount(0);
                setMood(3);
                setFulfillment(3);
                setBaseWeather('');
                setAmbientWeathers([]);
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
                base_weather: baseWeather,
                ambient_weathers: ambientWeathers,
            });
            if (res.data) {
                setLastSaved(res.data.updated_at ? parseISO(res.data.updated_at) : new Date());
                setEditCount(res.data.edit_count || 0);
                setMood(res.data.mood ?? mood);
                setFulfillment(res.data.fulfillment ?? fulfillment);
                setBaseWeather(isBaseWeatherValue(res.data.base_weather) ? res.data.base_weather : '');
                setAmbientWeathers(
                    Array.isArray(res.data.ambient_weathers)
                        ? res.data.ambient_weathers.filter(isAmbientWeatherValue)
                        : []
                );
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

    const toggleAmbientWeather = (value: AmbientWeatherValue) => {
        setAmbientWeathers((current) =>
            current.includes(value)
                ? current.filter((item) => item !== value)
                : ambientWeatherOptions
                      .map((option) => option.value)
                      .filter((option) => option === value || current.includes(option))
        );
    };

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

            <div className="space-y-4">
                <BaseWeatherControl value={baseWeather} onChange={setBaseWeather} />
                <AmbientWeatherControl values={ambientWeathers} onToggle={toggleAmbientWeather} />
            </div>

            {/* Editor */}
            <div className="bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-200 dark:border-gray-700 overflow-hidden min-h-[60vh] flex flex-col">
                <div className="relative">
                    <input
                        type="text"
                        value={title}
                        onChange={(e) => setTitle(e.target.value)}
                        placeholder="Title (optional)"
                        className="w-full px-6 py-4 text-xl font-semibold bg-transparent border-b border-gray-100 dark:border-gray-700 focus:outline-none dark:text-white"
                    />
                    <button
                        type="button"
                        onClick={regenerateTitle}
                        aria-label="Regenerate title"
                        title="重新生成标题"
                        className="absolute right-4 top-3 p-2 rounded-lg border border-gray-200 bg-white/80 dark:bg-gray-700/70 dark:border-gray-600 text-indigo-600 hover:bg-indigo-50 dark:hover:bg-indigo-700/30 hover:scale-105 transition transform shadow-sm"
                    >
                        <RefreshCw className="w-5 h-5" />
                    </button>
                    {/* toast moved to bottom-center */}
                </div>
                <textarea
                    value={content}
                    onChange={(e) => setContent(e.target.value)}
                    placeholder="Write your thoughts..."
                    className="flex-1 w-full p-6 resize-none focus:outline-none bg-transparent text-lg leading-relaxed dark:text-gray-200"
                />
                <div className="px-6 py-3 flex justify-end items-center gap-4 border-t border-gray-100 dark:border-gray-700">
                    <div className="text-xs text-gray-500 dark:text-gray-400">{displayCount} words</div>
                    <div className="text-xs text-gray-500 dark:text-gray-400">Edits {editCount}</div>
                </div>
            </div>
            {toastMessage && (
                <div className="fixed bottom-8 left-1/2 transform -translate-x-1/2 z-50 pointer-events-none">
                    <div className="inline-block bg-black/80 text-white text-sm px-4 py-2 rounded-md shadow-lg opacity-100 transition-all duration-200">
                        {toastMessage}
                    </div>
                </div>
            )}
        </div>
    );
}
