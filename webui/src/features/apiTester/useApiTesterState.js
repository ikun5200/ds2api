import { useEffect, useRef, useState } from 'react'

export function useApiTesterState({ t }) {
    const [model, setModel] = useState('deepseek-flash')
    const [thinkingEnabled, setThinkingEnabled] = useState(true)
    const [searchEnabled, setSearchEnabled] = useState(false)
    const defaultMessage = t('apiTester.defaultMessage')
    const [message, setMessage] = useState(defaultMessage)
    const [apiKey, setApiKey] = useState('')
    const [selectedAccount, setSelectedAccount] = useState('')
    const [response, setResponse] = useState(null)
    const [loading, setLoading] = useState(false)
    const [streamingContent, setStreamingContent] = useState('')
    const [streamingThinking, setStreamingThinking] = useState('')
    const [isStreaming, setIsStreaming] = useState(false)
    const [streamingMode, setStreamingMode] = useState(true)
    const [attachedFiles, setAttachedFiles] = useState([])
    const [configExpanded, setConfigExpanded] = useState(false)

    const abortControllerRef = useRef(null)
    const defaultMessageRef = useRef(defaultMessage)

    useEffect(() => {
        setMessage((prev) => (prev === defaultMessageRef.current ? defaultMessage : prev))
        defaultMessageRef.current = defaultMessage
    }, [defaultMessage])

    return {
        model,
        setModel,
        thinkingEnabled,
        setThinkingEnabled,
        searchEnabled,
        setSearchEnabled,
        message,
        setMessage,
        attachedFiles,
        setAttachedFiles,
        apiKey,
        setApiKey,
        selectedAccount,
        setSelectedAccount,
        response,
        setResponse,
        loading,
        setLoading,
        streamingContent,
        setStreamingContent,
        streamingThinking,
        setStreamingThinking,
        isStreaming,
        setIsStreaming,
        streamingMode,
        setStreamingMode,
        configExpanded,
        setConfigExpanded,
        abortControllerRef,
    }
}
