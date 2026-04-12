import re

with open('./frontend/src/app/student/session/[id]/page.tsx', 'r') as f:
    content = f.read()

page_changes = """
    const onError = useCallback((msg: string) => {
        toast(msg, 'error');
        router.push('/student/activities');
    }, [toast, router]);

    const onSessionLoaded = useCallback((sessionData: StudentSession) => {
         setScaffoldLevel(sessionData.scaffold_level || 'high');
    }, []);

    // Custom Hooks Integration
    const {
        messages,
        setMessages,
        addMessage,
        streamingContent,
        setStreamingContent,
        thinkingStatus,
        setThinkingStatus,
        sending,
        setSending,
        handleStreamingDelta,
        handleStreamingComplete,
        setPendingQuestion,
        session,
        setSession,
        activity,
        loading,
        autoStartTriggeredRef
    } = useSessionMessages({
        sessionId,
        onError,
        onSessionLoaded
    });
"""

content = re.sub(r"    // Custom Hooks Integration\n    const \{\n.*?        autoStartTriggeredRef\n    \} = useSessionMessages\(\{\n        sessionId,\n        onError: \(msg\) => \{\n            toast\(msg, 'error'\);\n            router\.push\('/student/activities'\);\n        \},\n        onSessionLoaded: \(sessionData\) => \{\n             setScaffoldLevel\(sessionData\.scaffold_level \|\| 'high'\);\n        \}\n    \}\);", page_changes, content, flags=re.DOTALL)


with open('./frontend/src/app/student/session/[id]/page.tsx', 'w') as f:
    f.write(content)
