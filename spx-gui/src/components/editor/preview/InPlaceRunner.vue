<script setup lang="ts">
import { ref, watch } from 'vue'
import { untilNotNull } from '@/utils/utils'
import ProjectRunner from '@/components/project/runner/ProjectRunner.vue'
import { useEditorCtx } from '../EditorContextProvider.vue'
import { RuntimeOutputKind } from '@/models/runtime'
import { getCleanupSignal } from '@/utils/disposable'

const props = defineProps<{
  visible: boolean
}>()

const editorCtx = useEditorCtx()
const projectRunnerRef = ref<InstanceType<typeof ProjectRunner>>()

function handleConsole(type: 'log' | 'warn', args: unknown[]) {
  if (type === 'log' && args.length === 1 && typeof args[0] === 'string') {
    try {
      const logMsg = JSON.parse(args[0])
      if (logMsg.level === 'ERROR' && logMsg.error && logMsg.msg === 'captured panic') {
        editorCtx.runtime.addOutput({
          kind: RuntimeOutputKind.Error,
          time: logMsg.time,
          message: logMsg.error,
          source: {
            textDocument: {
              uri: `file:///${logMsg.file}`
            },
            range: {
              start: { line: logMsg.line, column: logMsg.column },
              end: { line: logMsg.line, column: logMsg.column }
            }
          }
        })
        return
      }
    } catch {
      // If parsing fails, fall through to default handling.
    }
  }

  editorCtx.runtime.addOutput({
    kind: type === 'warn' ? RuntimeOutputKind.Error : RuntimeOutputKind.Log,
    time: Date.now(),
    message: args.join(' ')
  })
}

watch(
  () => props.visible,
  async (visible, _, onCleanup) => {
    if (!visible) return

    const signal = getCleanupSignal(onCleanup)
    const projectRunner = await untilNotNull(projectRunnerRef)
    signal.throwIfAborted()
    editorCtx.runtime.clearOutputs()
    projectRunner.run().then(() => {
      editorCtx.runtime.setRunning({ mode: 'debug', initializing: false })
    })
    signal.addEventListener('abort', () => {
      projectRunner.stop()
    })
  },
  { immediate: true }
)

defineExpose({
  async rerun() {
    editorCtx.runtime.setRunning({ mode: 'debug', initializing: true })
    const projectRunner = await untilNotNull(projectRunnerRef)
    editorCtx.runtime.clearOutputs()
    await projectRunner.rerun()
    editorCtx.runtime.setRunning({ mode: 'debug', initializing: false })
  }
})
</script>

<template>
  <ProjectRunner ref="projectRunnerRef" :project="editorCtx.project" @console="handleConsole" />
</template>

<style lang="scss" scoped></style>
