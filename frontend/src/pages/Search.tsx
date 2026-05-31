import React, { useState, useEffect, useMemo } from 'react';
import { Link } from 'react-router-dom';
import { format, parseISO } from 'date-fns';
import api from '../lib/api';
import { Search as SearchIcon, Calendar, Clock, Edit3, Heart, BatteryFull } from 'lucide-react';
import { TextHighlight } from '../components/TextHighlight';
import {
    ambientWeatherOptions,
    baseWeatherOptions,
    cn,
    getAmbientWeatherEmojis,
    getBaseWeatherEmoji,
    countContentUnits,
    type AmbientWeatherValue,
    type BaseWeatherValue,
} from '../lib/utils';

interface SearchEntry {
    id: number;
    title: string;
    content: string;
    day: string;
    created_at: string;
    updated_at: string;
    edit_count: number;
    mood: number;
    fulfillment: number;
    base_weather: string;
    ambient_weathers: string[];
}

export default function Search() {
    const [query, setQuery] = useState('');
    const [results, setResults] = useState<SearchEntry[]>([]);
    const [loading, setLoading] = useState(false);
    const [baseWeather, setBaseWeather] = useState<BaseWeatherValue | ''>('');
    const [ambientWeathers, setAmbientWeathers] = useState<AmbientWeatherValue[]>([]);
    const [moods, setMoods] = useState<number[]>([]);
    const [fulfillments, setFulfillments] = useState<number[]>([]);

    const ratingOptions = useMemo(() => [1, 2, 3, 4, 5], []);

    const hasFilters =
        baseWeather !== '' || ambientWeathers.length > 0 || moods.length > 0 || fulfillments.length > 0;
    const hasActiveSearch = query.trim().length > 0 || hasFilters;

    useEffect(() => {
        const timer = setTimeout(() => {
            if (hasActiveSearch) {
                performSearch();
            } else {
                setResults([]);
            }
        }, 500);
        return () => clearTimeout(timer);
    }, [query, baseWeather, ambientWeathers, moods, fulfillments, hasActiveSearch]);

    const toggleRating = (list: number[], value: number) =>
        list.includes(value) ? list.filter((item) => item !== value) : [...list, value].sort();

    const toggleAmbient = (value: AmbientWeatherValue) => {
        setAmbientWeathers((prev) =>
            prev.includes(value) ? prev.filter((item) => item !== value) : [...prev, value]
        );
    };

    const performSearch = async () => {
        setLoading(true);
        try {
            const params = new URLSearchParams();
            if (query.trim()) {
                params.set('q', query.trim());
            }
            if (baseWeather) {
                params.set('base_weather', baseWeather);
            }
            ambientWeathers.forEach((value) => params.append('ambient', value));
            moods.forEach((value) => params.append('mood', String(value)));
            fulfillments.forEach((value) => params.append('fulfillment', String(value)));
            const res = await api.get(`/search?${params.toString()}`);
            setResults(res.data || []);
        } catch (err) {
            console.error(err);
        } finally {
            setLoading(false);
        }
    };

    return (
        <div className="space-y-6">
            <h2 className="text-2xl font-bold text-gray-900 dark:text-white">Search</h2>

            <div className="relative">
                <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                    <SearchIcon className="h-5 w-5 text-gray-400" />
                </div>
                <input
                    type="text"
                    className="block w-full pl-10 pr-3 py-4 border border-gray-300 dark:border-gray-700 rounded-xl leading-5 bg-white dark:bg-gray-800 placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500 sm:text-sm dark:text-white"
                    placeholder="Search entries by text..."
                    value={query}
                    onChange={(e) => setQuery(e.target.value)}
                />
            </div>

            <div className="rounded-2xl border border-indigo-100/80 dark:border-indigo-500/20 bg-gradient-to-br from-indigo-50 via-white to-rose-50 dark:from-indigo-950/40 dark:via-gray-900 dark:to-rose-950/30 p-5 shadow-sm">
                <div className="flex flex-wrap items-center justify-between gap-3">
                    <div>
                        <p className="text-sm font-semibold text-indigo-700 dark:text-indigo-200">Filters</p>
                        <p className="text-xs text-gray-500 dark:text-gray-400">
                            Match any selected option within each group
                        </p>
                    </div>
                    <button
                        type="button"
                        onClick={() => {
                            setBaseWeather('');
                            setAmbientWeathers([]);
                            setMoods([]);
                            setFulfillments([]);
                        }}
                        className="text-xs font-medium text-gray-600 hover:text-indigo-600 dark:text-gray-300 dark:hover:text-indigo-300"
                    >
                        Clear all
                    </button>
                </div>

                <div className="mt-5 grid gap-4 lg:grid-cols-2">
                    <div className="rounded-xl border border-gray-200/80 dark:border-gray-700 bg-white/80 dark:bg-gray-900/60 p-4">
                        <div className="flex items-center justify-between">
                            <span className="text-sm font-medium text-gray-700 dark:text-gray-200">Base Weather</span>
                            <span className="text-xs text-gray-400">single</span>
                        </div>
                        <div className="mt-3 grid grid-cols-7 gap-2">
                            {baseWeatherOptions.map((option) => {
                                const active = baseWeather === option.value;
                                return (
                                    <button
                                        key={option.value}
                                        type="button"
                                        aria-label={option.label}
                                        aria-pressed={active}
                                        onClick={() => setBaseWeather(active ? '' : option.value)}
                                        className={cn(
                                            'flex h-12 items-center justify-center rounded-lg text-xl transition-all',
                                            active
                                                ? 'bg-white shadow-sm ring-1 ring-black/5 dark:bg-gray-800 dark:ring-white/10'
                                                : 'bg-gray-50 text-gray-500 hover:bg-white dark:bg-gray-800/60 dark:text-gray-300 dark:hover:bg-gray-700'
                                        )}
                                    >
                                        <span className="emoji-font">{option.emoji}</span>
                                    </button>
                                );
                            })}
                        </div>
                    </div>

                    <div className="rounded-xl border border-gray-200/80 dark:border-gray-700 bg-white/80 dark:bg-gray-900/60 p-4">
                        <div className="flex items-center justify-between">
                            <span className="text-sm font-medium text-gray-700 dark:text-gray-200">Ambient Weather</span>
                            <span className="text-xs text-gray-400">{ambientWeathers.length} selected</span>
                        </div>
                        <div className="mt-3 grid grid-cols-3 gap-2 sm:grid-cols-6">
                            {ambientWeatherOptions.map((option) => {
                                const active = ambientWeathers.includes(option.value);
                                return (
                                    <button
                                        key={option.value}
                                        type="button"
                                        aria-label={option.label}
                                        aria-pressed={active}
                                        onClick={() => toggleAmbient(option.value)}
                                        className={cn(
                                            'flex min-h-14 flex-col items-center justify-center gap-1 rounded-lg px-1 py-2 text-xs transition-all',
                                            active
                                                ? 'bg-white shadow-sm ring-1 ring-black/5 dark:bg-gray-800 dark:ring-white/10'
                                                : 'bg-gray-50 text-gray-500 hover:bg-white dark:bg-gray-800/60 dark:text-gray-300 dark:hover:bg-gray-700'
                                        )}
                                    >
                                        <span className="emoji-font text-lg leading-none">{option.emoji}</span>
                                        <span className="text-[11px] text-gray-600 dark:text-gray-300">{option.label}</span>
                                    </button>
                                );
                            })}
                        </div>
                    </div>

                    <div className="rounded-xl border border-gray-200/80 dark:border-gray-700 bg-white/80 dark:bg-gray-900/60 p-4">
                        <div className="flex items-center justify-between">
                            <span className="text-sm font-medium text-gray-700 dark:text-gray-200">Mood</span>
                            <span className="text-xs text-gray-400">{moods.length} selected</span>
                        </div>
                        <div className="mt-3 flex flex-wrap gap-2">
                            {ratingOptions.map((value) => {
                                const active = moods.includes(value);
                                return (
                                    <button
                                        key={value}
                                        type="button"
                                        aria-pressed={active}
                                        onClick={() => setMoods((prev) => toggleRating(prev, value))}
                                        className={cn(
                                            'min-w-10 rounded-full px-3 py-1.5 text-sm font-semibold transition-all',
                                            active
                                                ? 'bg-indigo-600 text-white shadow-sm'
                                                : 'bg-gray-100 text-gray-600 hover:bg-white dark:bg-gray-800 dark:text-gray-300 dark:hover:bg-gray-700'
                                        )}
                                    >
                                        {value}
                                    </button>
                                );
                            })}
                        </div>
                    </div>

                    <div className="rounded-xl border border-gray-200/80 dark:border-gray-700 bg-white/80 dark:bg-gray-900/60 p-4">
                        <div className="flex items-center justify-between">
                            <span className="text-sm font-medium text-gray-700 dark:text-gray-200">Fulfillment</span>
                            <span className="text-xs text-gray-400">{fulfillments.length} selected</span>
                        </div>
                        <div className="mt-3 flex flex-wrap gap-2">
                            {ratingOptions.map((value) => {
                                const active = fulfillments.includes(value);
                                return (
                                    <button
                                        key={value}
                                        type="button"
                                        aria-pressed={active}
                                        onClick={() => setFulfillments((prev) => toggleRating(prev, value))}
                                        className={cn(
                                            'min-w-10 rounded-full px-3 py-1.5 text-sm font-semibold transition-all',
                                            active
                                                ? 'bg-rose-500 text-white shadow-sm'
                                                : 'bg-gray-100 text-gray-600 hover:bg-white dark:bg-gray-800 dark:text-gray-300 dark:hover:bg-gray-700'
                                        )}
                                    >
                                        {value}
                                    </button>
                                );
                            })}
                        </div>
                    </div>
                </div>
            </div>

            <div className="space-y-4">
                {hasActiveSearch && !loading && (
                    <p className="text-sm text-gray-500 dark:text-gray-400">
                        Found {results.length} result{results.length !== 1 ? 's' : ''}
                    </p>
                )}

                {results.map((entry) => (
                    <Link
                        key={entry.id}
                        to={`/?date=${entry.day}`}
                        className="block bg-white dark:bg-gray-800 p-6 rounded-xl shadow-sm border border-gray-200 dark:border-gray-700 hover:border-indigo-500 dark:hover:border-indigo-500 transition-colors"
                    >
                        <div className="flex justify-between items-start mb-2">
                            <h3 className="text-lg font-semibold text-gray-900 dark:text-white">
                                <TextHighlight text={entry.title || 'Untitled'} query={query} />
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
                        <p className="text-gray-600 dark:text-gray-300 line-clamp-3">
                            <TextHighlight text={entry.content} query={query} />
                        </p>
                        <div className="mt-4 flex justify-end items-center gap-4 text-sm text-gray-500 dark:text-gray-400">
                            <div className="text-xs text-gray-500 dark:text-gray-400">{countContentUnits(entry.content)} words</div>
                            <span className="flex items-center gap-1">
                                <Edit3 className="w-4 h-4" />
                                {entry.edit_count}
                            </span>
                        </div>
                    </Link>
                ))}

                {hasActiveSearch && !loading && results.length === 0 && (
                    <div className="text-center py-12 text-gray-500">
                        {query.trim() ? `No results found for "${query}"` : 'No results found for selected filters'}
                    </div>
                )}
            </div>
        </div>
    );
}
