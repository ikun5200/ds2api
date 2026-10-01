import { createContext, useContext, useMemo } from 'react'
import zh from './locales/zh.json'

const translations = { zh }

const I18nContext = createContext({
    lang: 'zh',
    t: (key) => key,
})

const getValue = (obj, key) => {
    if (!obj) return undefined
    return key.split('.').reduce((acc, part) => (acc ? acc[part] : undefined), obj)
}

const formatMessage = (message, vars) => {
    if (!vars) return message
    return message.replace(/\{(\w+)\}/g, (match, key) => {
        if (Object.prototype.hasOwnProperty.call(vars, key)) {
            return vars[key]
        }
        return match
    })
}

export const I18nProvider = ({ children }) => {
    const t = useMemo(() => {
        return (key, vars) => {
            const value = getValue(translations.zh, key)
            if (typeof value !== 'string') return value
            return formatMessage(value, vars)
        }
    }, [])

    const contextValue = useMemo(() => ({ lang: 'zh', t }), [t])

    return <I18nContext.Provider value={contextValue}>{children}</I18nContext.Provider>
}

export const useI18n = () => useContext(I18nContext)
