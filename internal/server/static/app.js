// do-more dashboard app

// State
let config = null;
let providers = [];
let tasks = [];
let loopRunning = false;
let eventSource = null;

// Event type constants (must match server events.go)
const EventLoopStarted = 'loop_started';
const EventLoopCompleted = 'loop_completed';
const EventLoopError = 'loop_error';
const EventLoopStopped = 'loop_stopped';
const EventTaskStarted = 'task_started';
const EventIterationStarted = 'iteration_started';
const EventProviderInvoked = 'provider_invoked';
const EventProviderFinished = 'provider_finished';
const EventGateResult = 'gate_result';
const EventTaskDone = 'task_done';
const EventTaskFailed = 'task_failed';
const EventLogMessage = 'log_message';

// Status constants (must match config.go)
const StatusPending = 'pending';
const StatusInProgress = 'in_progress';
const StatusDone = 'done';
const StatusFailed = 'failed';

// Initialize on page load
document.addEventListener('DOMContentLoaded', function() {
    loadInitialData();
    connectEventSource();
});

// Load all initial data
async function loadInitialData() {
    await Promise.all([
        loadConfig(),
        loadProviders(),
        loadLoopStatus()
    ]);
}

// Load config and tasks
async function loadConfig() {
    try {
        const response = await fetch('/api/config');
        if (!response.ok) {
            throw new Error('Failed to load config');
        }
        config = await response.json();
        tasks = config.tasks || [];
        renderProjectInfo();
        renderTasks();
    } catch (error) {
        console.error('Error loading config:', error);
        document.getElementById('project-info').innerHTML = 
            '<span class="error-message">Error loading project info</span>';
        document.getElementById('tasks-list').innerHTML = 
            '<span class="error-message">Error loading tasks</span>';
    }
}

// Load providers
async function loadProviders() {
    try {
        const response = await fetch('/api/providers');
        if (!response.ok) {
            throw new Error('Failed to load providers');
        }
        providers = await response.json();
        renderProviders();
        populateProviderDropdowns();
    } catch (error) {
        console.error('Error loading providers:', error);
        document.getElementById('providers-list').innerHTML = 
            '<span class="error-message">Error loading providers</span>';
    }
}

// Load loop status
async function loadLoopStatus() {
    try {
        const response = await fetch('/api/loop/status');
        if (!response.ok) {
            throw new Error('Failed to load loop status');
        }
        const data = await response.json();
        loopRunning = data.running;
        updateLoopControls();
    } catch (error) {
        console.error('Error loading loop status:', error);
    }
}

// Connect to SSE stream
function connectEventSource() {
    if (eventSource) {
        eventSource.close();
    }

    eventSource = new EventSource('/api/events');

    eventSource.onmessage = function(event) {
        try {
            const data = JSON.parse(event.data);
            handleSSEEvent(data);
        } catch (error) {
            console.error('Error parsing SSE event:', error);
        }
    };

    eventSource.onerror = function(error) {
        console.error('SSE error:', error);
    };

    eventSource.onopen = function() {
        console.log('SSE connection opened');
        loadConfig();
        loadLoopStatus();
    };
}

// Handle SSE events
function handleSSEEvent(event) {
    appendEventToLog(event);

    switch (event.type) {
        case EventLoopStarted:
            loopRunning = true;
            updateLoopControls();
            break;
        case EventLoopStopped:
        case EventLoopCompleted:
        case EventLoopError:
            loopRunning = false;
            updateLoopControls();
            break;
        case EventTaskDone:
        case EventTaskFailed:
            loadConfig();
            break;
    }
}

// Append event to event log
function appendEventToLog(event) {
    const eventLog = document.getElementById('event-log');
    
    const eventItem = document.createElement('div');
    eventItem.className = 'event-item';
    
    const timestamp = event.timestamp ? formatTimestamp(event.timestamp) : formatTimestamp(new Date());
    const type = event.type || 'unknown';
    
    let dataHtml = '';
    if (event.data) {
        const dataStr = JSON.stringify(event.data);
        if (event.type === EventGateResult && event.data.passed !== undefined) {
            const passFail = event.data.passed ? 
                '<span class="event-pass">✓ PASS</span>' : 
                '<span class="event-fail">✗ FAIL</span>';
            dataHtml = `<span class="event-data">${escapeHtml(event.data.command || '')} ${passFail}</span>`;
        } else if (event.data.message) {
            dataHtml = `<span class="event-data">${escapeHtml(event.data.message)}</span>`;
        } else if (dataStr !== '{}') {
            dataHtml = `<span class="event-data">${escapeHtml(dataStr)}</span>`;
        }
    }
    
    if (event.taskId) {
        dataHtml = `<span class="event-data">[Task #${escapeHtml(event.taskId)}] ${dataHtml}</span>`;
    }
    
    eventItem.innerHTML = `
        <span class="event-timestamp">[${timestamp}]</span>
        <span class="event-type">${escapeHtml(type)}</span>
        ${dataHtml}
    `;
    
    eventLog.appendChild(eventItem);
    eventLog.scrollTop = eventLog.scrollHeight;
}

// Format timestamp for display
function formatTimestamp(timestamp) {
    const date = new Date(timestamp);
    return date.toLocaleTimeString('en-US', { 
        hour12: false, 
        hour: '2-digit', 
        minute: '2-digit', 
        second: '2-digit' 
    });
}

// Escape HTML to prevent XSS
function escapeHtml(text) {
    if (!text) return '';
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

// Render project info
function renderProjectInfo() {
    if (!config) return;
    
    const gatesHtml = config.gates && config.gates.length > 0 
        ? `<div class="gates-list">${config.gates.map(g => `<span class="gate-tag">${escapeHtml(g)}</span>`).join('')}</div>`
        : '<span class="text-muted">No gates configured</span>';
    
    document.getElementById('project-info').innerHTML = `
        <div class="project-info-item">
            <span class="project-info-label">Project:</span>
            <span class="project-info-value">${escapeHtml(config.name || 'Unknown')}</span>
        </div>
        <div class="project-info-item">
            <span class="project-info-label">Branch:</span>
            <span class="project-info-value">${escapeHtml(config.branch || 'Unknown')}</span>
        </div>
        <div class="project-info-item">
            <span class="project-info-label">Provider:</span>
            <span class="project-info-value">${escapeHtml(config.provider || 'Unknown')}</span>
        </div>
        <div class="project-info-item">
            <span class="project-info-label">Max Iterations:</span>
            <span class="project-info-value">${config.maxIterations || 'N/A'}</span>
        </div>
        <div class="project-info-item">
            <span class="project-info-label">Gates:</span>
            ${gatesHtml}
        </div>
    `;
}

// Render providers list
function renderProviders() {
    if (!config || !providers.length) return;
    
    const html = providers.map(provider => {
        const isDefault = provider === config.provider;
        const defaultClass = isDefault ? 'default' : '';
        const defaultLabel = isDefault ? ' (default)' : '';
        return `<span class="provider-badge ${defaultClass}">${escapeHtml(provider)}${defaultLabel}</span>`;
    }).join('');
    
    document.getElementById('providers-list').innerHTML = html;
}

// Populate provider dropdowns
function populateProviderDropdowns() {
    if (!config || !providers.length) return;
    
    const options = [
        `<option value="">Default (${escapeHtml(config.provider || 'none')})</option>`,
        ...providers.map(p => `<option value="${escapeHtml(p)}">${escapeHtml(p)}</option>`)
    ].join('');
    
    document.getElementById('task-provider').innerHTML = options;
    document.getElementById('edit-task-provider').innerHTML = options;
}

// Render tasks list
function renderTasks() {
    if (!tasks.length) {
        document.getElementById('tasks-list').innerHTML = '<div class="empty-state">No tasks yet. Create one above.</div>';
        return;
    }
    
    const html = tasks.map(task => {
        const statusClass = `status-${task.status}`;
        const providerDisplay = task.provider || `Default (${config.provider})`;
        
        return `
            <div class="task-item" data-task-id="${escapeHtml(task.id)}">
                <div class="task-header">
                    <div class="task-title-section">
                        <span class="task-id">#${escapeHtml(task.id)}</span>
                        <span class="task-title">${escapeHtml(task.title)}</span>
                    </div>
                    <div class="task-meta">
                        <span class="status-badge ${statusClass}">${escapeHtml(task.status)}</span>
                        <span class="task-provider">Provider: ${escapeHtml(providerDisplay)}</span>
                        <div class="task-actions">
                            <button class="btn btn-edit btn-small" onclick="openEditModal('${escapeHtml(task.id)}')" ${task.status === StatusInProgress ? 'disabled' : ''}>Edit</button>
                            <button class="btn btn-delete btn-small" onclick="deleteTask('${escapeHtml(task.id)}')" ${task.status === StatusInProgress ? 'disabled' : ''}>Delete</button>
                        </div>
                    </div>
                </div>
                ${task.description ? `<div class="task-description">${escapeHtml(task.description)}</div>` : ''}
            </div>
        `;
    }).join('');
    
    document.getElementById('tasks-list').innerHTML = html;
}

// Update loop controls UI
function updateLoopControls() {
    const statusIndicator = document.getElementById('loop-status-indicator');
    const btnStart = document.getElementById('btn-start');
    const btnStop = document.getElementById('btn-stop');
    const btnSkip = document.getElementById('btn-skip');
    
    if (loopRunning) {
        statusIndicator.textContent = 'Running';
        statusIndicator.className = 'status-badge status-running';
        btnStart.disabled = true;
        btnStop.disabled = false;
        btnSkip.disabled = false;
    } else {
        statusIndicator.textContent = 'Stopped';
        statusIndicator.className = 'status-badge status-stopped';
        btnStart.disabled = false;
        btnStop.disabled = true;
        btnSkip.disabled = true;
    }
}

// Start loop
async function startLoop() {
    clearError('loop-error');
    
    try {
        const response = await fetch('/api/loop/start', { method: 'POST' });
        const data = await response.json();
        
        if (!response.ok) {
            showError('loop-error', data.error || 'Failed to start loop');
            return;
        }
        
        loopRunning = true;
        updateLoopControls();
    } catch (error) {
        showError('loop-error', 'Network error: ' + error.message);
    }
}

// Stop loop
async function stopLoop() {
    clearError('loop-error');
    
    try {
        const response = await fetch('/api/loop/stop', { method: 'POST' });
        const data = await response.json();
        
        if (!response.ok) {
            showError('loop-error', data.error || 'Failed to stop loop');
            return;
        }
        
        loopRunning = false;
        updateLoopControls();
    } catch (error) {
        showError('loop-error', 'Network error: ' + error.message);
    }
}

// Skip current task
async function skipTask() {
    clearError('loop-error');
    
    try {
        const response = await fetch('/api/loop/skip', { method: 'POST' });
        const data = await response.json();
        
        if (!response.ok) {
            showError('loop-error', data.error || 'Failed to skip task');
            return;
        }
    } catch (error) {
        showError('loop-error', 'Network error: ' + error.message);
    }
}

// Create task
async function createTask(event) {
    event.preventDefault();
    clearError('create-task-error');
    
    const form = event.target;
    const formData = new FormData(form);
    
    const taskData = {
        title: formData.get('title'),
        description: formData.get('description'),
        provider: formData.get('provider') || ''
    };
    
    try {
        const response = await fetch('/api/tasks', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(taskData)
        });
        
        const data = await response.json();
        
        if (!response.ok) {
            showError('create-task-error', data.error || 'Failed to create task');
            return;
        }
        
        form.reset();
        loadConfig();
    } catch (error) {
        showError('create-task-error', 'Network error: ' + error.message);
    }
}

// Open edit modal
function openEditModal(taskId) {
    const task = tasks.find(t => t.id === taskId);
    if (!task) return;
    
    document.getElementById('edit-task-id').value = task.id;
    document.getElementById('edit-task-title').value = task.title;
    document.getElementById('edit-task-description').value = task.description || '';
    document.getElementById('edit-task-provider').value = task.provider || '';
    
    clearError('edit-task-error');
    document.getElementById('edit-modal').style.display = 'flex';
}

// Close edit modal
function closeEditModal() {
    document.getElementById('edit-modal').style.display = 'none';
}

// Update task
async function updateTask(event) {
    event.preventDefault();
    clearError('edit-task-error');
    
    const form = event.target;
    const formData = new FormData(form);
    
    const taskId = formData.get('id');
    const taskData = {
        title: formData.get('title'),
        description: formData.get('description'),
        provider: formData.get('provider') || ''
    };
    
    try {
        const response = await fetch(`/api/tasks/${encodeURIComponent(taskId)}`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(taskData)
        });
        
        const data = await response.json();
        
        if (!response.ok) {
            showError('edit-task-error', data.error || 'Failed to update task');
            return;
        }
        
        closeEditModal();
        loadConfig();
    } catch (error) {
        showError('edit-task-error', 'Network error: ' + error.message);
    }
}

// Delete task
async function deleteTask(taskId) {
    if (!confirm('Are you sure you want to delete this task?')) {
        return;
    }
    
    try {
        const response = await fetch(`/api/tasks/${encodeURIComponent(taskId)}`, {
            method: 'DELETE'
        });
        
        if (!response.ok) {
            const data = await response.json().catch(() => ({}));
            alert(data.error || 'Failed to delete task');
            return;
        }
        
        loadConfig();
    } catch (error) {
        alert('Network error: ' + error.message);
    }
}

// Show error message
function showError(elementId, message) {
    const element = document.getElementById(elementId);
    if (element) {
        element.textContent = message;
    }
}

// Clear error message
function clearError(elementId) {
    const element = document.getElementById(elementId);
    if (element) {
        element.textContent = '';
    }
}

// Close modal when clicking outside
document.addEventListener('click', function(event) {
    const modal = document.getElementById('edit-modal');
    if (event.target === modal) {
        closeEditModal();
    }
});
