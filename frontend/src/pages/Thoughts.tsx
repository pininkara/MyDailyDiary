import { useCallback, useEffect, useRef, useState } from 'react';
import { format, parseISO } from 'date-fns';
import { Check, Loader2, MessageSquareText, Pencil, Save, Trash2, X } from 'lucide-react';
import api from '../lib/api';

interface Thought {
    id: number;
    content: string;
    created_at: string;
    updated_at: string;
}

interface ThoughtPage {
    items: Thought[];
    next_cursor?: string;
    has_more: boolean;
}

const PAGE_SIZE = 20;

function sortThoughts(items: Thought[]) {
    return [...items].sort((left, right) => {
        const timeDifference = parseISO(right.updated_at).getTime() - parseISO(left.updated_at).getTime();
        return timeDifference || right.id - left.id;
    });
}

export default function Thoughts() {
    const [thoughts, setThoughts] = useState<Thought[]>([]);
    const [content, setContent] = useState('');
    const [loading, setLoading] = useState(true);
    const [loadingMore, setLoadingMore] = useState(false);
    const [saving, setSaving] = useState(false);
    const [hasMore, setHasMore] = useState(true);
    const [loadError, setLoadError] = useState('');
    const [actionError, setActionError] = useState('');
    const [editingID, setEditingID] = useState<number | null>(null);
    const [editContent, setEditContent] = useState('');
    const [updatingID, setUpdatingID] = useState<number | null>(null);
    const [deletingID, setDeletingID] = useState<number | null>(null);
    const sentinelRef = useRef<HTMLDivElement | null>(null);
    const cursorRef = useRef('');
    const loadingRef = useRef(false);

    const loadNextPage = useCallback(async (initial = false) => {
        if (loadingRef.current) return;

        loadingRef.current = true;
        if (initial) {
            setLoading(true);
        } else {
            setLoadingMore(true);
        }
        setLoadError('');

        try {
            const cursor = initial ? '' : cursorRef.current;
            const response = await api.get<ThoughtPage>('/thoughts', {
                params: {
                    limit: PAGE_SIZE,
                    ...(cursor ? { cursor } : {}),
                },
            });
            const page = response.data;
            setThoughts((current) => {
                const incoming = initial ? page.items : [...current, ...page.items];
                return incoming.filter(
                    (thought, index, all) => all.findIndex((item) => item.id === thought.id) === index
                );
            });
            cursorRef.current = page.next_cursor ?? '';
            setHasMore(page.has_more);
        } catch (error) {
            console.error('Failed to load thoughts', error);
            setLoadError('Could not load thoughts.');
        } finally {
            loadingRef.current = false;
            setLoading(false);
            setLoadingMore(false);
        }
    }, []);

    useEffect(() => {
        void loadNextPage(true);
    }, [loadNextPage]);

    useEffect(() => {
        const sentinel = sentinelRef.current;
        if (!sentinel || !hasMore || loading || loadingMore || loadError) return;

        const observer = new IntersectionObserver(
            (entries) => {
                if (entries[0]?.isIntersecting) {
                    void loadNextPage();
                }
            },
            { rootMargin: '240px 0px' }
        );
        observer.observe(sentinel);
        return () => observer.disconnect();
    }, [hasMore, loadError, loadNextPage, loading, loadingMore]);

    const handleCreate = async () => {
        const trimmedContent = content.trim();
        if (!trimmedContent || saving) return;

        setSaving(true);
        setActionError('');
        try {
            const response = await api.post<Thought>('/thoughts', { content: trimmedContent });
            setThoughts((current) => sortThoughts([response.data, ...current]));
            setContent('');
        } catch (error) {
            console.error('Failed to save thought', error);
            setActionError('Could not save your thought.');
        } finally {
            setSaving(false);
        }
    };

    const beginEditing = (thought: Thought) => {
        setEditingID(thought.id);
        setEditContent(thought.content);
        setActionError('');
    };

    const cancelEditing = () => {
        setEditingID(null);
        setEditContent('');
    };

    const handleUpdate = async (id: number) => {
        const trimmedContent = editContent.trim();
        if (!trimmedContent || updatingID !== null) return;

        setUpdatingID(id);
        setActionError('');
        try {
            const response = await api.put<Thought>(`/thoughts/${id}`, { content: trimmedContent });
            setThoughts((current) =>
                sortThoughts(current.map((thought) => (thought.id === id ? response.data : thought)))
            );
            cancelEditing();
        } catch (error) {
            console.error('Failed to update thought', error);
            setActionError('Could not update this thought.');
        } finally {
            setUpdatingID(null);
        }
    };

    const handleDelete = async (thought: Thought) => {
        if (!window.confirm('Delete this thought? This cannot be undone.')) return;

        setDeletingID(thought.id);
        setActionError('');
        try {
            await api.delete(`/thoughts/${thought.id}`);
            setThoughts((current) => current.filter((item) => item.id !== thought.id));
            if (editingID === thought.id) {
                cancelEditing();
            }
        } catch (error) {
            console.error('Failed to delete thought', error);
            setActionError('Could not delete this thought.');
        } finally {
            setDeletingID(null);
        }
    };

    return (
        <div className="space-y-8">
            <h2 className="text-2xl font-bold text-gray-900 dark:text-white">Thoughts</h2>

            <section className="overflow-hidden rounded-lg border border-gray-200 bg-white shadow-sm dark:border-gray-700 dark:bg-gray-800">
                <textarea
                    value={content}
                    onChange={(event) => setContent(event.target.value)}
                    placeholder="What's on your mind?"
                    aria-label="New thought"
                    rows={6}
                    className="block min-h-40 w-full resize-y bg-transparent px-5 py-4 text-base leading-7 text-gray-900 outline-none placeholder:text-gray-400 dark:text-gray-100"
                />
                <div className="flex items-center justify-between gap-4 border-t border-gray-100 px-4 py-3 dark:border-gray-700">
                    <span className="text-xs text-gray-400">{content.length} characters</span>
                    <button
                        type="button"
                        onClick={handleCreate}
                        disabled={!content.trim() || saving}
                        className="flex min-h-10 items-center gap-2 rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-indigo-700 disabled:cursor-not-allowed disabled:opacity-50"
                    >
                        {saving ? <Loader2 className="h-4 w-4 animate-spin" /> : <Save className="h-4 w-4" />}
                        {saving ? 'Saving...' : 'Save thought'}
                    </button>
                </div>
            </section>

            {actionError && (
                <div role="alert" className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900 dark:bg-red-950/40 dark:text-red-300">
                    {actionError}
                </div>
            )}

            <section aria-label="Saved thoughts" className="space-y-4">
                {loading ? (
                    <div className="flex min-h-48 items-center justify-center text-gray-500">
                        <Loader2 className="h-6 w-6 animate-spin" aria-label="Loading thoughts" />
                    </div>
                ) : (
                    thoughts.map((thought) => {
                        const isEditing = editingID === thought.id;
                        const isUpdating = updatingID === thought.id;
                        const isDeleting = deletingID === thought.id;

                        return (
                            <article
                                key={thought.id}
                                className="rounded-lg border border-gray-200 bg-white p-5 shadow-sm dark:border-gray-700 dark:bg-gray-800"
                            >
                                {isEditing ? (
                                    <textarea
                                        value={editContent}
                                        onChange={(event) => setEditContent(event.target.value)}
                                        aria-label="Edit thought"
                                        rows={5}
                                        autoFocus
                                        className="block min-h-32 w-full resize-y rounded-lg border border-gray-300 bg-white px-4 py-3 leading-7 text-gray-900 outline-none focus:border-indigo-500 focus:ring-2 focus:ring-indigo-500/20 dark:border-gray-600 dark:bg-gray-900 dark:text-gray-100"
                                    />
                                ) : (
                                    <p className="whitespace-pre-wrap break-words leading-7 text-gray-800 dark:text-gray-200">
                                        {thought.content}
                                    </p>
                                )}

                                <footer className="mt-4 flex min-h-9 flex-wrap items-center justify-between gap-3 border-t border-gray-100 pt-3 dark:border-gray-700">
                                    <time
                                        dateTime={thought.updated_at}
                                        className="text-xs text-gray-500 dark:text-gray-400"
                                    >
                                        Saved {format(parseISO(thought.updated_at), "MMM d, yyyy 'at' HH:mm")}
                                    </time>

                                    <div className="flex items-center gap-1">
                                        {isEditing ? (
                                            <>
                                                <button
                                                    type="button"
                                                    onClick={() => void handleUpdate(thought.id)}
                                                    disabled={!editContent.trim() || isUpdating}
                                                    aria-label="Save changes"
                                                    title="Save changes"
                                                    className="flex h-9 w-9 items-center justify-center rounded-lg text-indigo-600 transition-colors hover:bg-indigo-50 disabled:opacity-50 dark:text-indigo-400 dark:hover:bg-indigo-950/50"
                                                >
                                                    {isUpdating ? <Loader2 className="h-4 w-4 animate-spin" /> : <Check className="h-4 w-4" />}
                                                </button>
                                                <button
                                                    type="button"
                                                    onClick={cancelEditing}
                                                    disabled={isUpdating}
                                                    aria-label="Cancel editing"
                                                    title="Cancel editing"
                                                    className="flex h-9 w-9 items-center justify-center rounded-lg text-gray-500 transition-colors hover:bg-gray-100 disabled:opacity-50 dark:text-gray-400 dark:hover:bg-gray-700"
                                                >
                                                    <X className="h-4 w-4" />
                                                </button>
                                            </>
                                        ) : (
                                            <button
                                                type="button"
                                                onClick={() => beginEditing(thought)}
                                                disabled={editingID !== null || isDeleting}
                                                aria-label="Edit thought"
                                                title="Edit thought"
                                                className="flex h-9 w-9 items-center justify-center rounded-lg text-gray-500 transition-colors hover:bg-gray-100 disabled:opacity-50 dark:text-gray-400 dark:hover:bg-gray-700"
                                            >
                                                <Pencil className="h-4 w-4" />
                                            </button>
                                        )}
                                        <button
                                            type="button"
                                            onClick={() => void handleDelete(thought)}
                                            disabled={isUpdating || isDeleting}
                                            aria-label="Delete thought"
                                            title="Delete thought"
                                            className="flex h-9 w-9 items-center justify-center rounded-lg text-gray-500 transition-colors hover:bg-red-50 hover:text-red-600 disabled:opacity-50 dark:text-gray-400 dark:hover:bg-red-950/40 dark:hover:text-red-400"
                                        >
                                            {isDeleting ? <Loader2 className="h-4 w-4 animate-spin" /> : <Trash2 className="h-4 w-4" />}
                                        </button>
                                    </div>
                                </footer>
                            </article>
                        );
                    })
                )}

                {!loading && thoughts.length === 0 && !loadError && (
                    <div className="flex min-h-48 flex-col items-center justify-center gap-3 text-center text-gray-500 dark:text-gray-400">
                        <MessageSquareText className="h-8 w-8" />
                        <p>No thoughts yet.</p>
                    </div>
                )}

                {loadError && (
                    <div role="alert" className="flex flex-col items-center gap-3 py-8 text-center text-sm text-gray-600 dark:text-gray-300">
                        <p>{loadError}</p>
                        <button
                            type="button"
                            onClick={() => void loadNextPage(thoughts.length === 0)}
                            className="rounded-lg border border-gray-300 bg-white px-4 py-2 font-medium text-gray-700 transition-colors hover:bg-gray-50 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-200 dark:hover:bg-gray-700"
                        >
                            Try again
                        </button>
                    </div>
                )}

                <div ref={sentinelRef} className="h-1" aria-hidden="true" />

                {loadingMore && (
                    <div className="flex items-center justify-center gap-2 py-5 text-sm text-gray-500 dark:text-gray-400">
                        <Loader2 className="h-4 w-4 animate-spin" />
                        Loading more...
                    </div>
                )}
            </section>
        </div>
    );
}
