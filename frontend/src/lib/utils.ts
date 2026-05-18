import { type ClassValue, clsx } from "clsx"
import { twMerge } from "tailwind-merge"

export function cn(...inputs: ClassValue[]) {
    return twMerge(clsx(inputs))
}

export type BaseWeatherValue = 'sunny' | 'cloudy' | 'overcast' | 'light_rain' | 'storm' | 'snow' | 'other'
export type AmbientWeatherValue = 'fog' | 'windy' | 'hot' | 'cold' | 'rainbow' | 'extreme'

export const baseWeatherOptions: Array<{ value: BaseWeatherValue; emoji: string; label: string }> = [
    { value: 'sunny', emoji: '☀️', label: '晴天' },
    { value: 'cloudy', emoji: '⛅', label: '多云' },
    { value: 'overcast', emoji: '☁️', label: '阴天' },
    { value: 'light_rain', emoji: '🌧', label: '小雨' },
    { value: 'storm', emoji: '⛈️', label: '暴雨' },
    { value: 'snow', emoji: '❄️', label: '下雪' },
    { value: 'other', emoji: '✨', label: '其他' },
]

export const ambientWeatherOptions: Array<{ value: AmbientWeatherValue; emoji: string; label: string }> = [
    { value: 'fog', emoji: '🌫️', label: 'Fog' },
    { value: 'windy', emoji: '💨', label: 'Windy' },
    { value: 'hot', emoji: '🥵', label: 'Hot' },
    { value: 'cold', emoji: '🥶', label: 'Cold' },
    { value: 'rainbow', emoji: '🌈', label: 'Rainbow' },
    { value: 'extreme', emoji: '🌪️', label: 'Extreme' },
]

const baseWeatherValues = new Set(baseWeatherOptions.map(option => option.value))
const ambientWeatherValues = new Set(ambientWeatherOptions.map(option => option.value))

export function isBaseWeatherValue(value: string): value is BaseWeatherValue {
    return baseWeatherValues.has(value as BaseWeatherValue)
}

export function isAmbientWeatherValue(value: string): value is AmbientWeatherValue {
    return ambientWeatherValues.has(value as AmbientWeatherValue)
}

export function getBaseWeatherEmoji(value: string) {
    return baseWeatherOptions.find(option => option.value === value)?.emoji ?? ''
}

export function getAmbientWeatherEmojis(values: string[]) {
    return ambientWeatherOptions
        .filter(option => values.includes(option.value))
        .map(option => option.emoji)
}
