<script setup lang="ts">
import { Icon } from "@iconify/vue";
import { VButton, VCard, VSwitch } from "@/components/ui";
import PluginsTab from "@/modules/settings/PluginsTab.vue";
import IconPicker from "@/components/IconPicker.vue";

const props = defineProps<{ data: Record<string, any>; saving: string }>();
const emit = defineEmits<{ save: [section: string] }>();
const colors = ["red", "orange", "yellow", "green", "cyan", "blue", "purple"];
const inputCls = "apple-input";
const defaultStatusNames = ["in_track", "at_risk", "off_track", "finished", "unfinished"];
const emojiSources = ["default", "twemoji", "qq", "aru", "tieba", "blobcat"];

function saveTagPlugins() {
  emit("save", "tag_plugins");
}

function saveServices() {
  emit("save", "services");
}

function statusNames() {
  return Object.keys(props.data.tag_plugins.okr.status || {});
}

function renameStatus(oldName: string, event: Event) {
  const input = event.target as HTMLInputElement;
  const newName = input.value.trim();
  const statuses = props.data.tag_plugins.okr.status;
  if (!newName || (newName !== oldName && Object.prototype.hasOwnProperty.call(statuses, newName))) {
    input.value = oldName;
    return;
  }
  if (newName === oldName) return;
  statuses[newName] = statuses[oldName];
  delete statuses[oldName];
}

function addStatus() {
  const statuses = props.data.tag_plugins.okr.status;
  let name = "custom";
  let index = 1;
  while (Object.prototype.hasOwnProperty.call(statuses, name)) {
    index += 1;
    name = `custom_${index}`;
  }
  statuses[name] = { color: "blue", label: "自定义状态" };
}

function removeStatus(name: string) {
  if (!defaultStatusNames.includes(name)) delete props.data.tag_plugins.okr.status[name];
}
</script>

<template>
  <VCard title="前端增强">
    <PluginsTab :data="data.plugins" :saving="saving" @save="emit('save', 'plugins')" />
  </VCard>

  <VCard title="内容组件默认参数">
    <p class="settings-hint">
      对齐 Stellar <code>tag_plugins</code> 段：未在具体标签中指定的参数使用这里的默认值。复杂或未覆盖的 shortcode
      仍可通过原始内容无损保留。
    </p>

    <div class="settings-subsection settings-subsection-first">
      <p class="settings-subsection-title">默认配色</p>
      <div class="settings-form-grid settings-form-grid-3">
        <div
          v-for="item in [
            ['note', 'note 提示框'],
            ['mark', 'mark 荧光标记'],
            ['button', 'button 按钮'],
            ['icon', 'icon 图标'],
            ['hashtag', 'hashtag 话题'],
          ]"
          :key="item[0]"
          class="settings-field"
        >
          <label>{{ item[1] }}</label>
          <select v-model="data.tag_plugins[item[0]].default_color" class="apple-select">
            <option value="">{{ item[0] === "hashtag" ? "循环" : "默认" }}</option>
            <option v-for="color in colors" :key="color" :value="color">{{ color }}</option>
          </select>
        </div>
      </div>
    </div>

    <div class="settings-subsection">
      <p class="settings-subsection-title">交互与边框</p>
      <div class="settings-switch-row">
        <div>
          <div class="settings-switch-name">note 显示边框</div>
          <div class="settings-switch-desc">控制提示框外围的边框；标签自身显式参数仍优先。</div>
        </div>
        <VSwitch v-model="data.tag_plugins.note.border" aria-label="note 显示边框" />
      </div>
      <div class="settings-switch-row">
        <div>
          <div class="settings-switch-name">checkbox 允许交互</div>
          <div class="settings-switch-desc">允许访客直接勾选 checkbox/radio 标签。</div>
        </div>
        <VSwitch v-model="data.tag_plugins.checkbox.interactive" aria-label="checkbox 允许交互" />
      </div>
      <div class="settings-switch-row">
        <div>
          <div class="settings-switch-name">OKR 显示边框</div>
          <div class="settings-switch-desc">控制 OKR 外围边框，状态颜色和文案见下方状态表。</div>
        </div>
        <VSwitch v-model="data.tag_plugins.okr.border" aria-label="OKR 显示边框" />
      </div>
    </div>

    <div class="settings-subsection">
      <p class="settings-subsection-title">quot 图标预设</p>
      <p class="settings-hint">
        shortcode 中使用 <code>icon:default</code>、<code>icon:hashtag</code> 或
        <code>icon:question</code> 选择预设；显式 prefix/suffix 优先。
      </p>
      <div v-for="name in ['default', 'hashtag', 'question']" :key="name" class="settings-nested-group">
        <p class="settings-subsection-title">{{ name }}</p>
        <div class="settings-form-grid settings-form-grid-2">
          <div class="settings-field">
            <label :for="`settings-quot-${name}-prefix`">前缀</label>
            <IconPicker
              :id="`settings-quot-${name}-prefix`"
              v-model="data.tag_plugins.quot[name].prefix"
              placeholder="如 quot:quote-left"
            />
          </div>
          <div class="settings-field">
            <label :for="`settings-quot-${name}-suffix`">后缀</label>
            <IconPicker
              :id="`settings-quot-${name}-suffix`"
              v-model="data.tag_plugins.quot[name].suffix"
              placeholder="如 quot:quote-right"
            />
          </div>
        </div>
      </div>
    </div>

    <div class="settings-subsection">
      <p class="settings-subsection-title">OKR 状态</p>
      <p class="settings-hint">
        内置状态用于完成度回退；可以编辑颜色和文案，也可以增加自定义状态。状态名用于 shortcode 的
        <code>status:</code> 参数。
      </p>
      <div class="settings-editable-list">
        <div v-for="name in statusNames()" :key="name" class="settings-editable-row settings-okr-status-row">
          <input
            :value="name"
            :aria-label="`${name} 状态名`"
            :class="inputCls"
            class="settings-col-3"
            @change="renameStatus(name, $event)"
          />
          <input
            v-model="data.tag_plugins.okr.status[name].color"
            placeholder="颜色"
            :class="inputCls"
            class="settings-col-3"
          />
          <input
            v-model="data.tag_plugins.okr.status[name].label"
            placeholder="文案"
            :class="inputCls"
            class="settings-col-4"
          />
          <button
            v-if="!defaultStatusNames.includes(name)"
            type="button"
            class="settings-row-btn settings-row-btn-danger"
            aria-label="删除自定义状态"
            @click="removeStatus(name)"
          >
            <Icon icon="ri:delete-bin-line" />
          </button>
        </div>
      </div>
      <button type="button" class="settings-add-btn" @click="addStatus">
        <Icon icon="ri:add-line" />添加自定义状态
      </button>
    </div>

    <div class="settings-subsection">
      <p class="settings-subsection-title">gallery 默认布局</p>
      <div class="settings-form-grid settings-form-grid-3">
        <div class="settings-field">
          <label>布局</label
          ><select v-model="data.tag_plugins.gallery.layout" class="apple-select">
            <option value="">默认</option>
            <option value="grid">grid</option>
            <option value="flow">flow</option>
          </select>
        </div>
        <div class="settings-field">
          <label>尺寸</label><input v-model="data.tag_plugins.gallery.size" :class="inputCls" placeholder="mix" />
        </div>
        <div class="settings-field">
          <label>宽高比</label><input v-model="data.tag_plugins.gallery.ratio" :class="inputCls" placeholder="origin" />
        </div>
      </div>
    </div>

    <div class="settings-subsection">
      <p class="settings-subsection-title">其它</p>
      <div class="settings-form-grid settings-form-grid-2">
        <div class="settings-field">
          <label>timeline 限高（如 80vh，空=不限）</label
          ><input v-model="data.tag_plugins.timeline.max_height" :class="inputCls" />
        </div>
        <div class="settings-field">
          <label>copy 成功提示文案</label
          ><input v-model="data.tag_plugins.copy.toast" :class="inputCls" placeholder="复制成功" />
        </div>
      </div>
    </div>

    <div class="settings-subsection">
      <p class="settings-subsection-title">emoji 表情源（{name} 占位）</p>
      <div class="settings-form-grid settings-form-grid-2">
        <div v-for="name in emojiSources" :key="name" class="settings-field">
          <label :for="`settings-emoji-${name}`">{{ name }}</label>
          <input :id="`settings-emoji-${name}`" v-model="data.tag_plugins.emoji[name]" :class="inputCls" />
        </div>
      </div>
    </div>

    <div class="settings-card-actions">
      <VButton type="primary" size="sm" :loading="saving === 'tag_plugins'" @click="saveTagPlugins">
        <template #icon><Icon icon="ri:save-line" /></template>保存内容组件设置
      </VButton>
    </div>
  </VCard>

  <VCard title="SiteInfo 服务">
    <p class="settings-hint">
      为缺少 icon/avatar 的站点卡片请求图标兜底。URL 中使用 <code>{href}</code> 占位符，留空表示关闭。
    </p>
    <div class="settings-field">
      <label for="settings-siteinfo-api">SiteInfo API 模板</label>
      <input
        id="settings-siteinfo-api"
        v-model="data.services.siteinfo.api"
        :class="inputCls"
        placeholder="https://api.example.com/siteinfo?url={href}"
      />
    </div>
    <div class="settings-card-actions">
      <VButton type="primary" size="sm" :loading="saving === 'services'" @click="saveServices">
        <template #icon><Icon icon="ri:save-line" /></template>保存 SiteInfo 设置
      </VButton>
    </div>
  </VCard>
</template>
