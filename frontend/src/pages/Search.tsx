import React, { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { format, parseISO } from 'date-fns';
import api from '../lib/api';
import { Search as SearchIcon, Calendar, Clock, Edit3, Heart, BatteryFull } from 'lucide-react';
import { TextHighlight } from '../components/TextHighlight';
import { getAmbientWeatherEmojis, getBaseWeatherEmoji } from '../lib/utils';

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

    useEffect(() => {
        const timer = setTimeout(() => {
            if (query.trim()) {
                performSearch();
            } else {
                setResults([]);
            }
        }, 500);
        return () => clearTimeout(timer);
    }, [query]);

    const performSearch = async () => {
        setLoading(true);
        try {
            const res = await api.get(`/search?q=${encodeURIComponent(query)}`);
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

            <div className="space-y-4">
                {query && !loading && (
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
                                        <span className="text-base leading-none">
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
                                <span className="flex items-center gap-1">
                                    <Edit3 className="w-4 h-4" />
                                    {entry.edit_count}
                                </span>
                            </div>
                        </div>
                        <p className="text-gray-600 dark:text-gray-300 line-clamp-3">
                            <TextHighlight text={entry.content} query={query} />
                        </p>
                    </Link>
                ))}

                {query && !loading && results.length === 0 && (
                    <div className="text-center py-12 text-gray-500">
                        No results found for "{query}"
                    </div>
                )}
            </div>
        </div>
    );
}
