function makeFloaters() {
    // IE 11 doesn't render this correctly, but maybe I'll fix it later.
    let emojis = ['🤔','️🤔','🤔','🤔','🤔','🤔','🤔','🤔','🤔','🤔',
                  '🤔','🤔','🤔','🤔','🤔','🤔','🤔','🤔','🤔','🤔'];

    // Fewer emojis on smaller screens.
    if (screen.width < 1000) {
        emojis.length = emojis.length/2;
    }

    let floaters = [];
    emojis.forEach(function(e) {
        let child = document.createElement('span');
        child.classList.add('floater');
        child.innerHTML = e;
        floaters.push(child);
        document.getElementById('emoji-background').appendChild(child);
    });

    let i=1;
    let animCss = '';
    floaters.forEach(function(floater) {
        let topStart = Math.round(Math.random() * 100);
        let topEnd = Math.round(Math.random() * 100);
        let duration = Math.round(Math.random() * 20 + 8);
        let delay = Math.round(Math.random() * 20);
        let rotationStart = Math.round(Math.random() * 360);
        let rotationEnd = Math.round(Math.random() * 360 + 360);
        let leftStrings = ['left: -120px;\n', 'left: calc(100vw + 120px);\n'];
        if (Math.random() > 0.5) {
            leftStrings.unshift(leftStrings.pop());
        }

        floater.classList.add('floater-' + i);
        animCss += '@keyframes emoji-float-' + i + ' ' +
            '{ \n' +
            '  from {\n' +
            '    ' + leftStrings.pop() +
            '    top: ' + topStart + 'vh;\n' +
            '    transform: rotateZ(' + rotationStart + 'deg);\n' +
            '  }\n' +
            '  to {\n' +
            '    ' + leftStrings.pop() +
            '    top: ' + topEnd + 'vh;\n' +
            '    transform: rotateZ(' + rotationEnd + 'deg);\n' +
            '  }\n' +
            '}\n';

        animCss += '#emoji-background .floater-' + i + ' ' +
            '{ \n' +
            '  animation: emoji-float-' + i + ' ' +
            duration + 's ' +
            delay + 's linear infinite alternate; \n' +
            '}\n';
        i++;
    });
    let styleElement = document.createElement('style');
    styleElement.innerHTML = animCss;
    document.body.appendChild(styleElement);
}

let contentHolder = document.getElementById('content-holder');
let uploadContent = document.getElementById('upload-content');
let listContent = document.getElementById('list-content');
let soundList = document.getElementById('soundList');
let uploadButton = document.getElementById('upload-button');
let manageButton = document.getElementById('manage-button');
let newAlias = document.getElementById('new-alias');
let aliasList = document.getElementById('alias-list');
let file = document.getElementById('file');
let command = document.getElementById('command');

function setUpload() {
    uploadContent.style.display = 'block';
    listContent.style.display = 'none';
    getAliases()
}

function setList() {
    //Retrieve and load list of sounds, then show
    soundList.innerHTML = '';

    let request = new XMLHttpRequest();
    request.open('GET', '/get');
    request.send();
    request.onload = function (event) {
        soundList.innerHTML = this.response;
        uploadContent.style.display = 'none';
        listContent.style.display = 'block';
    }
}

uploadButton.addEventListener('click', setUpload, false);
manageButton.addEventListener('click', setList, false);

function getAliases() {
    let request = new XMLHttpRequest();
    request.open('GET', '/aliases');
    request.send();
    request.onload = function() {
        aliasList.innerHTML = '<option></option>'+this.response;
    }
}

function createSound() {
    let request = new XMLHttpRequest();
    request.open('POST', '/create');
    let formData = new FormData();
    formData.append('file', file.files[0]);
    formData.append('command', command.value);
    request.send(formData);
    request.onload = function() {
        command.value = "";
        file.value = "";
    }
}

function deleteSound(element) {
    let id = element.id.substr(0, element.id.indexOf('-'));
    let request = new XMLHttpRequest();
    request.open('POST', '/delete');
    let formData = new FormData();
    formData.append("delete", id);
    request.send(formData);
    request.onload = function() {
        setList()
    }
}

function createAlias() {
    let request = new XMLHttpRequest();

    let formData = new FormData();
    formData.append("newAlias", newAlias.value);
    formData.append("sound", aliasList.value);
    request.open('POST', '/createAlias');
    request.send(formData);
    request.onload = function() {
        aliasList.value = ""
    }
}



contentHolder.appendChild(uploadContent);
contentHolder.appendChild(listContent);

makeFloaters();
setUpload();
getAliases();
