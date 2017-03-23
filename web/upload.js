function makeFloaters() {
    // IE 11 doesn't render this correctly, but maybe I'll fix it later.
    var emojis = ['🤔','️🤔','🤔','🤔','🤔','🤔','🤔','🤔','🤔','🤔',
                  '🤔','🤔','🤔','🤔','🤔','🤔','🤔','🤔','🤔','🤔'];
    var template = '<span class="floater">%e</span>';

    // Fewer emojis on smaller screens.
    if ($(window).width() < 1000) {
      emojis.length = emojis.length/2;
    }

    emojis.forEach(function(e) {
      $('#emoji-background').append(template.replace('%e', e));
    });

    var i=1;
    var animCss = '<style>\n';
    $('#emoji-background .floater').each(function() {
      var topStart = Math.round(Math.random() * 100);
      var topEnd = Math.round(Math.random() * 100);
      var duration = Math.round(Math.random() * 20 + 8);
      var delay = Math.round(Math.random() * 20);
      var rotationStart = Math.round(Math.random() * 360);
      var rotationEnd = Math.round(Math.random() * 360 + 360);
      var leftStrings = ['left: -120px;\n', 'left: calc(100vw + 120px);\n'];
      if (Math.random() > 0.5) {
        leftStrings.unshift(leftStrings.pop());
      }

      $(this).addClass('floater-' + i);
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
    animCss += '</style>';
    $(document.body).append(animCss);
  }
