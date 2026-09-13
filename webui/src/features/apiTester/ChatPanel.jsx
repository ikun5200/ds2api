import { Bot, Loader2, Send, Square, User, Zap, Paperclip, X, FileIcon, ImageIcon } from 'lucide-react'
import clsx from 'clsx'
import { useRef, useState } from 'react'

import { bindAttachedFileCredential, getAttachedFileAccountIds, hasAttachmentCredentialMismatch } from './fileAccountBinding'

export default function ChatPanel({
    t,
    message,
    setMessage,
    attachedFiles = [],
    setAttachedFiles,
    setSelectedAccount,
    effectiveKey,
    usesManagedKey = false,
    selectedAccount,
    model,
    thinkingEnabled,
    onMessage,
    response,
    isStreaming,
    loading,
    streamingThinking,
    streamingContent,
    onRunTest,
    onStopGeneration,
    hasAvailableModel,
}) {
    const fileInputRef = useRef(null)
    const [uploadingFiles, setUploadingFiles] = useState(false)

    const handleFileSelect = async (e) => {
        const files = Array.from(e.target.files)
        e.target.value = ''
        if (files.length === 0) return

        if (!effectiveKey) {
            onMessage('error', t('apiTester.missingApiKey'))
            return
        }
        if (hasAttachmentCredentialMismatch(attachedFiles, effectiveKey, usesManagedKey)) {
            onMessage('error', t('apiTester.fileCredentialMismatch'))
            return
        }

        const initialSelectedAccount = String(selectedAccount || '').trim()
        const attachedAccountIds = getAttachedFileAccountIds(attachedFiles)
        if (attachedAccountIds.length > 1) {
            onMessage('error', t('apiTester.fileAccountConflict'))
            return
        }
        const attachedAccount = attachedAccountIds[0] || ''
        if (attachedAccount && initialSelectedAccount && initialSelectedAccount !== attachedAccount) {
            onMessage('error', t('apiTester.fileAccountMismatch', { account: attachedAccount }))
            return
        }

        setUploadingFiles(true)
        const selectedModel = String(model || '').trim()
        let boundAccount = initialSelectedAccount || attachedAccount
        for (const file of files) {
            const formData = new FormData()
            formData.append('file', file)
            formData.append('purpose', 'assistants')
            formData.append('thinking_enabled', String(thinkingEnabled))
            if (selectedModel) {
                formData.append('model', selectedModel)
            }

            const headers = {
                'Authorization': `Bearer ${effectiveKey}`,
            }
            if (boundAccount) {
                headers['X-Ds2-Target-Account'] = boundAccount
            }

            try {
                const res = await fetch('/v1/files', {
                    method: 'POST',
                    headers,
                    body: formData
                })
                if (!res.ok) {
                    const err = await res.text()
                    onMessage('error', err || t('apiTester.uploadFailed'))
                    continue
                }
                const data = await res.json()
                const attachment = bindAttachedFileCredential({ ...data, mime_type: data.mime_type || file.type }, effectiveKey, usesManagedKey)
                setAttachedFiles(prev => [...prev, attachment])
                const uploadedAccount = String(data?.account_id || '').trim()
                if (!boundAccount && uploadedAccount) {
                    boundAccount = uploadedAccount
                }
            } catch (error) {
                onMessage('error', error.message || t('apiTester.uploadFailed'))
            }
        }
        setUploadingFiles(false)
        if (!initialSelectedAccount && boundAccount && setSelectedAccount) {
            setSelectedAccount(current => String(current || '').trim() ? current : boundAccount)
        }
    }

    const removeFile = (id) => {
        setAttachedFiles(prev => prev.filter(f => f.id !== id))
    }

    const attachmentAccountIds = getAttachedFileAccountIds(attachedFiles)
    const attachmentAccountId = attachmentAccountIds.length === 1 ? attachmentAccountIds[0] : ''
    const credentialMismatch = hasAttachmentCredentialMismatch(attachedFiles, effectiveKey, usesManagedKey)
    return (
        <div className="lg:col-span-9 flex flex-col bg-card border border-border rounded-xl shadow-sm overflow-hidden min-h-0 flex-1 relative">
            <div className="flex-1 overflow-y-auto p-4 lg:p-6 space-y-8 custom-scrollbar scroll-smooth">
                <div className="flex gap-4 max-w-4xl mx-auto flex-row-reverse group">
                    <div className="w-8 h-8 rounded-lg bg-secondary flex items-center justify-center shrink-0 border border-border">
                        <User className="w-4 h-4 text-muted-foreground" />
                    </div>
                    <div className="space-y-1 max-w-[85%] lg:max-w-[75%]">
                        <div className="bg-primary text-primary-foreground rounded-2xl rounded-tr-sm px-5 py-3 text-sm leading-relaxed shadow-sm">
                            {message}
                        </div>
                    </div>
                </div>

                {(response || isStreaming) && (
                    <div className="flex gap-4 max-w-4xl mx-auto animate-in fade-in slide-in-from-bottom-2 duration-300">
                        <div className={clsx(
                            "w-8 h-8 rounded-lg flex items-center justify-center shrink-0 border border-border",
                            response?.success !== false ? "bg-muted" : "bg-destructive/10 border-destructive/20"
                        )}>
                            <Bot className={clsx("w-4 h-4", response?.success !== false ? "text-foreground" : "text-destructive")} />
                        </div>
                        <div className="space-y-3 flex-1 min-w-0">
                            <div className="flex items-center gap-2">
                                <span className="font-semibold text-sm text-foreground">DeepSeek</span>
                                {response && (
                                    <span className={clsx(
                                        "text-[10px] px-1.5 py-0.5 rounded-sm border uppercase font-medium tracking-wider",
                                        response.success ? "border-emerald-500/20 text-emerald-500 bg-emerald-500/10" : "border-destructive/20 text-destructive bg-destructive/10"
                                    )}>
                                        {response.status_code || t('apiTester.statusError')}
                                    </span>
                                )}
                            </div>

                            {(streamingThinking || response?.choices?.[0]?.message?.reasoning_content) && (
                                <div className="text-xs bg-secondary/50 border border-border rounded-lg p-3 space-y-1.5">
                                    <div className="flex items-center gap-1.5 text-muted-foreground">
                                        <Zap className="w-3.5 h-3.5" />
                                        <span className="font-medium">{t('apiTester.reasoningTrace')}</span>
                                    </div>
                                    <div className="whitespace-pre-wrap leading-relaxed text-muted-foreground font-mono text-[11px] max-h-60 overflow-y-auto custom-scrollbar pl-5 border-l-2 border-border/50">
                                        {streamingThinking || response?.choices?.[0]?.message?.reasoning_content}
                                    </div>
                                </div>
                            )}

                            <div className="text-sm leading-7 text-foreground whitespace-pre-wrap">
                                {response?.success === false
                                    ? <span className="text-destructive font-medium">{response.error || t('apiTester.requestFailed')}</span>
                                    : (streamingContent || response?.choices?.[0]?.message?.content || (loading && <span className="text-muted-foreground italic">{t('apiTester.generating')}</span>))}
                                {isStreaming && <span className="inline-block w-1.5 h-4 bg-primary ml-1 align-middle animate-pulse" />}
                            </div>
                        </div>
                    </div>
                )}
            </div>

            <div className="p-4 lg:p-6 border-t border-border bg-card">
                {attachedFiles.length > 0 && (
                    <div className="max-w-4xl mx-auto flex flex-wrap gap-2 mb-3">
                        {attachedFiles.map(file => (
                            <div key={file.id} className="flex items-center gap-2 bg-secondary/50 border border-border rounded-md px-2 py-1 text-xs text-secondary-foreground">
                                {file.mime_type?.startsWith('image/')
                                    ? <ImageIcon className="w-3 h-3 text-muted-foreground" />
                                    : <FileIcon className="w-3 h-3 text-muted-foreground" />}
                                <span className="truncate max-w-[150px]">{file.filename || file.id}</span>
                                <button
                                    type="button"
                                    aria-label={t('apiTester.removeAttachment', { filename: file.filename || file.id })}
                                    onClick={() => removeFile(file.id)}
                                    className="text-muted-foreground hover:text-destructive transition-colors ml-1"
                                >
                                    <X className="w-3 h-3" />
                                </button>
                            </div>
                        ))}
                    </div>
                )}
                {attachmentAccountIds.length > 1 && (
                    <div className="max-w-4xl mx-auto mb-3 text-[11px] text-amber-600">
                        {t('apiTester.fileAccountConflict')}
                    </div>
                )}
                {credentialMismatch && (
                    <div className="max-w-4xl mx-auto mb-3 text-[11px] text-amber-600">
                        {t('apiTester.fileCredentialMismatch')}
                    </div>
                )}
                {attachmentAccountId && (
                    <div className="max-w-4xl mx-auto mb-3 text-[11px] text-muted-foreground">
                        {t('apiTester.attachmentAccountHint', { account: attachmentAccountId })}
                    </div>
                )}
                <div className="max-w-4xl mx-auto relative group">
                    <input
                        type="file"
                        className="hidden"
                        ref={fileInputRef}
                        multiple
                        onChange={handleFileSelect}
                    />
                    <div className="absolute left-2 bottom-2 z-10">
                        <button
                            type="button"
                            onClick={() => fileInputRef.current?.click()}
                            disabled={uploadingFiles || isStreaming || !hasAvailableModel}
                            className="p-2 text-muted-foreground hover:text-primary transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                            title={t('apiTester.attachFiles')}
                            aria-label={t('apiTester.attachFiles')}
                        >
                            {uploadingFiles ? <Loader2 className="w-4 h-4 animate-spin" /> : <Paperclip className="w-4 h-4" />}
                        </button>
                    </div>
                    <textarea
                        className="w-full bg-card border border-border rounded-lg pl-12 pr-12 py-3 text-sm focus:ring-2 focus:ring-primary/20 focus:border-primary transition-all resize-none custom-scrollbar placeholder:text-muted-foreground/60 text-foreground shadow-sm disabled:opacity-60 disabled:cursor-not-allowed"
                        placeholder={hasAvailableModel ? t('apiTester.enterMessage') : t('apiTester.noModelsMessagePlaceholder')}
                        rows={1}
                        style={{ minHeight: '52px' }}
                        value={message}
                        disabled={!hasAvailableModel}
                        onChange={e => setMessage(e.target.value)}
                        onKeyDown={e => {
                            if (e.key === 'Enter' && !e.shiftKey) {
                                e.preventDefault()
                                if (!loading && !uploadingFiles && !credentialMismatch && (message.trim() || attachedFiles.length > 0)) {
                                    onRunTest()
                                }
                            }
                        }}
                    />
                    <div className="absolute right-2 bottom-2 z-10">
                        {loading && isStreaming ? (
                            <button onClick={onStopGeneration} className="p-2 text-muted-foreground hover:text-destructive transition-colors">
                                <Square className="w-4 h-4 fill-current" />
                            </button>
                        ) : (
                            <button
                                onClick={onRunTest}
                                disabled={loading || uploadingFiles || credentialMismatch || !hasAvailableModel || (!message.trim() && attachedFiles.length === 0)}
                                className="p-2 text-primary hover:text-primary/80 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                            >
                                {loading ? <Loader2 className="w-4 h-4 animate-spin" /> : <Send className="w-4 h-4" />}
                            </button>
                        )}
                    </div>
                </div>
                <div className="max-w-4xl mx-auto mt-3 flex justify-center">
                    <span className="text-[10px] text-muted-foreground/40 font-medium">{t('apiTester.adminConsoleLabel')}</span>
                </div>
            </div>
        </div>
    )
}
